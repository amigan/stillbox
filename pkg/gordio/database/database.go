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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Conn = *pgxpool.Pool

func NewClient(conf config.DB) (Conn, error) {
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

	db, err := pgxpool.New(context.Background(), conf.Connect)
	if err != nil {
		return nil, err
	}

	return db, nil
}

type DBCtxKey string

const DBCTXKeyValue DBCtxKey = "dbctx"

func Tx(ctx context.Context) pgx.Tx {
	c, ok := ctx.Value(DBCTXKeyValue).(pgx.Tx)
	if !ok {
		panic("no DB in context")
	}

	return c
}

func CtxWithTx(ctx context.Context, conn pgx.Tx) context.Context {
	return context.WithValue(ctx, DBCTXKeyValue, conn)
}
