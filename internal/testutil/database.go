package testutil

import (
	"context"
	"os"
	"path"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/database/partman"
	"dynatron.me/x/stillbox/testdata"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

type DB struct {
	*database.Postgres
	SchemaName string
	PartConfig config.Partition

	nowFunc NowFunc
}

func (db *DB) Cleanup() {
	_, err := db.Exec(context.Background(), "DROP SCHEMA "+db.SchemaName+" CASCADE;")
	if err != nil {
		panic(err)
	}

	db.Close()
}

type PartConfig func(schema string) config.Partition

func DailyPartConfig() PartConfig {
	return func(schemaName string) config.Partition {
		return config.Partition{
			Enabled:  true,
			Schema:   schemaName,
			Interval: "daily",
		}
	}
}

func CustomPartConfig(cfg config.Partition) PartConfig {
	return func(schemaName string) config.Partition {
		cfg.Schema = schemaName
		return cfg
	}
}

type DBOpt func(*dbOpts)
type NowFunc func() time.Time

type dbOpts struct {
	nowFunc NowFunc
	pc      PartConfig
}

func WithNow(nf NowFunc) DBOpt {
	return func(db *dbOpts) {
		db.nowFunc = nf
	}
}

func WithPartConfig(pc PartConfig) DBOpt {
	return func(db *dbOpts) {
		db.pc = pc
	}
}

func (db *DB) NowFunc() NowFunc {
	return db.nowFunc
}

func NewDB(opts ...DBOpt) *DB {
	primeBlobs()

	_ = godotenv.Load(path.Join(testdata.Path, "../.env.test"))

	ctx := context.Background()
	dbConnect := os.Getenv("STILLBOX_TESTDB_CONNECT")
	if dbConnect == "" {
		panic("no test database connect string provided")
	}
	logQueries := os.Getenv("STILLBOX_LOG_QUERIES")
	schemaName := "sb_test_" + common.RandSeq(16)

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

	var dbo dbOpts
	for _, opt := range opts {
		opt(&dbo)
	}

	var part config.Partition
	if dbo.pc != nil {
		part = dbo.pc(schemaName)
	}

	db, err := database.NewClient(ctx, config.DB{
		Connect:    dbConnect + "&search_path=" + schemaName,
		LogQueries: logQueries == "true",
		Partition:  part,
	})
	if err != nil {
		panic(err)
	}

	if dbo.pc == nil {
		_, err := db.Exec(ctx, "CREATE TABLE calls_default PARTITION OF calls DEFAULT;")
		if err != nil {
			panic(err)
		}
	}

	tdb := &DB{Postgres: db, SchemaName: schemaName, PartConfig: part, nowFunc: time.Now}

	if dbo.nowFunc != nil {
		tdb.nowFunc = dbo.nowFunc
	}

	if part.Enabled {
		pm, err := partman.NewPartitionManager(db, nil, part)
		if err != nil {
			panic(err)
		}

		err = pm.Check(ctx, tdb.nowFunc().UTC())
		if err != nil {
			panic(err)
		}
	}

	err = tdb.loadFixtures(ctx)
	if err != nil {
		panic(err)
	}

	return tdb
}
