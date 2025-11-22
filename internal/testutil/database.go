package testutil

import (
	"context"
	"math/rand"
	"os"

	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/testdata"
	"github.com/go-testfixtures/testfixtures/v3"

	"github.com/jackc/pgx/v5"
	pgxstd "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

type DB struct {
	*database.Postgres
	fixt       *testfixtures.Loader
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

func NewDB(part config.Partition) DB {
	_ = godotenv.Load(testdata.Path + "/../.env.test")

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

	db, err := database.NewClient(ctx, config.DB{
		Connect:    dbConnect + "&search_path=" + schemaName,
		LogQueries: logQueries == "true",
		Partition:  part,
	})
	if err != nil {
		panic(err)
	}

	stdDb := pgxstd.OpenDBFromPool(db.Pool)
	fixt, err := testfixtures.New(
		testfixtures.Database(stdDb),
		testfixtures.Dialect("postgres"),
		testfixtures.Directory(testdata.Path+"/fixtures"),
		testfixtures.UseAlterConstraint(),
	)
	if err != nil {
		panic(err)
	}

	err = fixt.Load()
	if err != nil {
		panic(err)
	}

	tdb := DB{Postgres: db, fixt: fixt, SchemaName: schemaName}

	return tdb
}
