package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dynatron.me/x/stillbox/pkg/config"
	sqlembed "dynatron.me/x/stillbox/sql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/rs/zerolog/log"
)

// DB is a database handle.

//go:generate mockery
type Store interface {
	Querier
	talkgroupQuerier
	partitionsQuerier

	DB() *Postgres
	DBTX() DBTX
	InTx(context.Context, func(Store) error, pgx.TxOptions) error
}

type Postgres struct {
	*pgxpool.Pool
	*Queries
}

func (q *Queries) DBTX() DBTX {
	return q.db
}

func (db *Postgres) DB() *Postgres {
	return db
}

func (db *Postgres) InTx(ctx context.Context, f func(Store) error, opts pgx.TxOptions) error {
	tx, err := db.DB().Pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("Tx begin: %w", err)
	}

	//nolint:errcheck
	defer tx.Rollback(ctx)

	dbtx := &Postgres{Pool: db.Pool, Queries: db.Queries.WithTx(tx)}

	err = f(dbtx)
	if err != nil {
		return fmt.Errorf("Tx: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("Tx commit: %w", err)
	}

	return nil
}

type dbLogger struct{}

func (m dbLogger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	var rs string

	// zerolog doesn't do this for us...
	if i, ok := data["args"].([]interface{}); ok {
		res := make([]string, 0, len(i))
		for _, v := range i {
			res = append(res, fmt.Sprintf("%T:%+v", v, v))
		}

		rs = strings.Join(res, ", ")
		data["args"] = "ARRAY:[" + rs + "]"
	}

	log.Debug().Fields(data).Msg(msg)
}

func Close(c Store) {
	c.(*Postgres).Pool.Close()
}

// NewClient creates a new DB using the provided config.
func NewClient(ctx context.Context, conf config.DB) (*Postgres, error) {
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

	if !errors.Is(err, migrate.ErrNoChange) {
		log.Info().Err(err).Msg("migrations done")
	}

	m.Close()

	pgConf, err := pgxpool.ParseConfig(conf.Connect)
	if err != nil {
		return nil, err
	}

	if conf.LogQueries {
		pgConf.ConnConfig.Tracer = &tracelog.TraceLog{
			Logger:   dbLogger{},
			LogLevel: tracelog.LogLevelTrace,
		}
	}

	pool, err := pgxpool.NewWithConfig(ctx, pgConf)
	if err != nil {
		return nil, err
	}

	db := &Postgres{
		Pool:    pool,
		Queries: New(pool),
	}

	return db, nil
}

type dBCtxKey string

const DBCtxKey dBCtxKey = "dbctx"

// FromCtx returns the database handle from the provided Context.
func FromCtx(ctx context.Context) Store {
	c, ok := ctx.Value(DBCtxKey).(Store)
	if !ok {
		panic("no DB in context")
	}

	return c
}

// CtxWithDB returns a Context with the provided database handle.
func CtxWithDB(ctx context.Context, conn Store) context.Context {
	return context.WithValue(ctx, DBCtxKey, conn)
}

// IsNoRows is a convenience function that returns whether a returned error is a database
// no rows error.
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
