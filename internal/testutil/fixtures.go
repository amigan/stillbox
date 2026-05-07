package testutil

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/users"
	"dynatron.me/x/stillbox/testdata"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

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

func SmallMP3() []byte {
	smallMP3, err := os.ReadFile(path.Join(testdata.Path, "small.mp3"))
	if err != nil {
		panic(err)
	}

	return smallMP3
}

func webpushSubscription(s string) string {
	d := struct {
		Endpoint string `json:"endpoint"`
	}{
		Endpoint: s,
	}
	ma, err := json.Marshal(&d)
	if err != nil {
		panic(err)
	}

	return "!!binary \"" + base64.StdEncoding.EncodeToString(ma) + "\""
}

func primeBlobs() {
	dmOnce.Do(func() {
		smallMP3 := SmallMP3()

		templateData = map[string]any{
			"smallMP3": "!!binary \"" + base64.StdEncoding.EncodeToString(smallMP3) + "\"",
		}
		templateFuncs = template.FuncMap{
			"uuid":        uidList.getUUID,
			"time":        uidList.getTime,
			"pwhash":      users.HashPassword,
			"webpush_sub": webpushSubscription,
			"now": func() string {
				return time.Now().Format(time.RFC3339Nano)
			},
		}
	})
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

	qry.WriteString(") OVERRIDING SYSTEM VALUE VALUES (")
	for i := 1; i < len(args)+1; i++ {
		qry.WriteString("$" + strconv.Itoa(i))
		if i < len(args) {
			qry.WriteRune(',')
		}
	}
	qry.WriteString(");")

	_, err := db.Exec(ctx, qry.String(), args...)
	if err != nil {
		return err
	}

	return nil
}

type typePair struct {
	name           string
	obj            any
	identityColumn *string
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

	if tp.identityColumn != nil {
		_, err := db.Exec(ctx, fmt.Sprintf("SELECT setval(pg_get_serial_sequence('%s', '%s'), (SELECT MAX(%s)+1 FROM %s), false);", tp.name, *tp.identityColumn, *tp.identityColumn, tp.name))
		return err
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
		{"users", database.User{}, common.PtrTo("id")},
		{"systems", database.System{}, nil},
		{"talkgroups", database.Talkgroup{}, common.PtrTo("id")},
		{"calls", database.Call{}, nil},
		{"incidents", database.Incident{}, nil},
		{"incidents_calls", database.IncidentsCall{}, nil},
		{"system_notification_subscriptions", database.SystemNotificationSubscription{}, nil},
		{"talkgroup_notification_subscriptions", database.TalkgroupNotificationSubscription{}, nil},
		{"webpush_subscriptions", database.WebpushSubscription{}, common.PtrTo("id")},
	}

	for _, table := range tps {
		err := db.loadTable(ctx, tmpl, table)
		if err != nil {
			return fmt.Errorf("%s: %w", table.name, err)
		}
	}

	return nil
}

func UUID(s string) string {
	return uidList.getUUID(s)
}
