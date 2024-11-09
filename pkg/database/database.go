package database

import (
	"context"
	"errors"
	"strings"

	"dynatron.me/x/stillbox/pkg/config"
	sqlembed "dynatron.me/x/stillbox/sql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is a database handle.
type DB struct {
	*pgxpool.Pool
	*Queries
}

// NewClient creates a new DB using the provided config.
func NewClient(ctx context.Context, conf config.DB) (*DB, error) {
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

	pgConf, err := pgxpool.ParseConfig(conf.Connect)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, pgConf)
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

const DBCtxKeyValue DBCtxKey = "dbctx"

// FromCtx returns the database handle from the provided Context.
func FromCtx(ctx context.Context) *DB {
	c, ok := ctx.Value(DBCtxKeyValue).(*DB)
	if !ok {
		panic("no DB in context")
	}

	return c
}

// CtxWithDB returns a Context with the provided database handle.
func CtxWithDB(ctx context.Context, conn *DB) context.Context {
	return context.WithValue(ctx, DBCtxKeyValue, conn)
}

// IsNoRows is a convenience function that returns whether a returned error is a database
// no rows error.
func IsNoRows(err error) bool {
	return strings.Contains(err.Error(), "no rows in result set")
}
