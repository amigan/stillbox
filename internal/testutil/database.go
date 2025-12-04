package testutil

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/database/partman"
	"dynatron.me/x/stillbox/testdata"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

type DB struct {
	*database.Postgres
	SchemaName string
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func (db DB) Cleanup() {
	_, err := db.Exec(context.Background(), "DROP SCHEMA "+db.SchemaName+" CASCADE;")
	if err != nil {
		panic(err)
	}

	db.Close()
}

var dmOnce sync.Once
var templateData map[string]any

type idList struct {
	sync.Mutex
	uuids map[string]string
	times map[string]time.Time
}

var uidList idList

func (u *idList) getUUID(label string) string {
	u.Lock()
	defer u.Unlock()

	if u.uuids == nil {
		u.uuids = make(map[string]string)
	}

	if ex, has := u.uuids[label]; has {
		return ex
	}

	uid := uuid.New().String()
	u.uuids[label] = uid

	return uid
}

func (u *idList) getTime(label string) string {
	u.Lock()
	defer u.Unlock()

	if u.times == nil {
		u.times = make(map[string]time.Time)
	}

	if ex, has := u.times[label]; has {
		return ex.Format(time.RFC3339Nano)
	}

	tm := time.Now()
	u.times[label] = tm
	return tm.Format(time.RFC3339Nano)
}

var templateFuncs template.FuncMap

func primeBlobs() {
	dmOnce.Do(func() {
		smallMP3, err := os.ReadFile(path.Join(testdata.Path, "small.mp3"))
		if err != nil {
			panic(err)
		}

		templateData = map[string]any{
			"smallMP3": "!!binary \"" + base64.StdEncoding.EncodeToString(smallMP3) + "\"",
		}
		templateFuncs = template.FuncMap{
			"uuid": uidList.getUUID,
			"time": uidList.getTime,
			"now": func() string {
				return time.Now().Format(time.RFC3339Nano)
			},
		}
	})
}

func UUID(s string) string {
	return uidList.getUUID(s)
}

func NewDB(part config.Partition) DB {
	primeBlobs()

	_ = godotenv.Load(path.Join(testdata.Path, "../.env.test"))

	ctx := context.Background()
	dbConnect := os.Getenv("STILLBOX_TESTDB_CONNECT")
	if dbConnect == "" {
		panic("no test database connect string provided")
	}
	logQueries := os.Getenv("STILLBOX_LOG_QUERIES")
	schemaName := "sb_test_" + randSeq(16)

	schemaConn, err := pgx.Connect(ctx, dbConnect)
	if err != nil {
		panic(err)
	}

	_, err = schemaConn.Exec(ctx, "CREATE SCHEMA "+schemaName+";")
	if err != nil {
		panic(err)
	}

	err = schemaConn.Close(ctx)
	if err != nil {
		panic(err)
	}

	part.Schema = schemaName
	db, err := database.NewClient(ctx, config.DB{
		Connect:    dbConnect + "&search_path=" + schemaName,
		LogQueries: logQueries == "true",
		Partition:  part,
	})
	if err != nil {
		panic(err)
	}

	if part.Enabled {
		pm, err := partman.NewPartitionManager(db, nil, part)
		if err != nil {
			panic(err)
		}

		err = pm.Check(ctx, time.Now().UTC())
		if err != nil {
			panic(err)
		}
	}

	tdb := DB{Postgres: db, SchemaName: schemaName}

	err = tdb.loadFixtures(ctx)
	if err != nil {
		panic(err)
	}

	return tdb
}

func load(tmpl *template.Template, name string, dst any) error {
	var w bytes.Buffer
	err := tmpl.ExecuteTemplate(&w, name+".yml", templateData)
	if err != nil {
		return err
	}

	err = yaml.NewDecoder(&w).Decode(dst)
	if err != nil {
		return err
	}

	return nil
}

func (db *DB) doInsert(ctx context.Context, table string, src any) error {
	rv := reflect.ValueOf(src)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return errors.New("not a struct")
	}

	var qry strings.Builder

	rt := rv.Type()
	nf := rt.NumField()

	args := make([]any, 0, nf)

	qry.WriteString("INSERT INTO ")
	qry.WriteString(table)
	qry.WriteString(" (")

	for i := range nf {
		t, has := rt.Field(i).Tag.Lookup("db")
		if !has {
			continue
		}
		ts := strings.Split(t, ",")
		qry.WriteString(ts[0])

		if i < nf-1 {
			qry.WriteRune(',')
		}

		args = append(args, rv.Field(i).Interface())
	}

	qry.WriteString(" ) OVERRIDING SYSTEM VALUE VALUES (")
	for i := 1; i < len(args)+1; i++ {
		qry.WriteString("$" + strconv.Itoa(i))
		if i < len(args) {
			qry.WriteRune(',')
		}
	}
	qry.WriteString(" );")

	_, err := db.Exec(ctx, qry.String(), args...)
	if err != nil {
		return err
	}

	if table == "talkgroups" {
		_, err := db.Exec(ctx, "SELECT setval(pg_get_serial_sequence('talkgroups', 'id'), (SELECT MAX(id)+1 FROM talkgroups), false);")
		return err
	}

	return nil
}

type typePair struct {
	name string
	obj  any
}

func (db *DB) loadTable(ctx context.Context, tmpl *template.Template, tp typePair) error {
	strType := reflect.TypeOf(tp.obj)

	vsl := reflect.MakeSlice(reflect.SliceOf(strType), 0, 1)

	slPtr := reflect.New(vsl.Type())
	slPtr.Elem().Set(vsl)
	err := load(tmpl, tp.name, slPtr.Interface())
	if err != nil {
		return err
	}

	vsl = slPtr.Elem()

	for i := range vsl.Len() {
		err := db.doInsert(ctx, tp.name, vsl.Index(i).Interface())
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) loadFixtures(ctx context.Context) error {
	tmpl, err := template.New("fixtures").Funcs(templateFuncs).ParseGlob(testdata.Path + "/fixtures/*.yml")
	if err != nil {
		return err
	}

	tmpl = tmpl.Funcs(templateFuncs)

	tps := []typePair{
		{"users", database.User{}},
		{"systems", database.System{}},
		{"talkgroups", database.Talkgroup{}},
		{"calls", database.Call{}},
		{"incidents", database.Incident{}},
		{"incidents_calls", database.IncidentsCall{}},
	}

	for _, table := range tps {
		err := db.loadTable(ctx, tmpl, table)
		if err != nil {
			return fmt.Errorf("%s: %w", table.name, err)
		}
	}

	return nil
}
