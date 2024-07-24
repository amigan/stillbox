package database

import (
	"context"
	"errors"
	"strings"

	"dynatron.me/x/stillbox/pkg/gordio/config"
	sqlembed "dynatron.me/x/stillbox/sql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	*pgxpool.Pool
	*Queries
}

func NewClient(conf config.DB) (*DB, error) {
	dir, err := iofs.New(sqlembed.Migrations, "postgres/migrations")
	if err != nil {
		return nil, err
	}

	m, err := migrate.NewWithSourceInstance("iofs", dir, strings.Replace(conf.Connect, "postgres://", "pgx5://", 1)) // yech
	if err != nil {
		return nil, err
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return nil, err
	}

	m.Close()

	pool, err := pgxpool.New(context.Background(), conf.Connect)
	if err != nil {
		return nil, err
	}

	db := &DB{
		Pool:    pool,
		Queries: New(pool),
	}

	return db, nil
}

type DBCtxKey string

const DBCTXKeyValue DBCtxKey = "dbctx"

func FromCtx(ctx context.Context) *DB {
	c, ok := ctx.Value(DBCTXKeyValue).(*DB)
	if !ok {
		panic("no DB in context")
	}

	return c
}

func CtxWithDB(ctx context.Context, conn *DB) context.Context {
	return context.WithValue(ctx, DBCTXKeyValue, conn)
}

func IsNoRows(err error) bool {
	return strings.Contains(err.Error(), "no rows in result set")
}
