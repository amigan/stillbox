package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/services"
	sqlembed "dynatron.me/x/stillbox/sql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	callsQuerier

	DB() *Postgres
	DBTX() DBTX
	InTx(context.Context, func(Store) error) error
	PoolTx(ctx context.Context, opts pgx.TxOptions) (dbtx *Postgres, tx pgx.Tx, err error)

	// DropTable drops the specified table.
	DropTable(ctx context.Context, tableName string) error
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

func (q *Queries) DropTable(ctx context.Context, tableName string) error {
	_, err := q.db.Exec(ctx, "DROP TABLE "+tableName+";")
	return err
}

func (db *Postgres) PoolTx(ctx context.Context, opts pgx.TxOptions) (dbtx *Postgres, tx pgx.Tx, err error) {
	tx, err = db.DB().Pool.BeginTx(ctx, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("Tx begin: %w", err)
	}

	dbtx = &Postgres{Pool: db.Pool, Queries: db.Queries.WithTx(tx)}

	return dbtx, tx, nil
}

func (db *Postgres) InTx(ctx context.Context, f func(Store) error) error {
	dbtx, tx, err := db.PoolTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer tx.Rollback(ctx)

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
	if i, ok := data["args"].([]any); ok {
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
		return nil, fmt.Errorf("iofs create: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", dir, strings.Replace(conf.Connect, "postgres://", "pgx5://", 1)) // yech
	if err != nil {
		return nil, fmt.Errorf("new migrate: %w", err)
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	if !errors.Is(err, migrate.ErrNoChange) {
		log.Info().Err(err).Msg("migrations done")
	}

	serr, dberr := m.Close()
	if serr != nil || dberr != nil {
		return nil, multierror.Append(serr, dberr)
	}

	pgConf, err := pgxpool.ParseConfig(conf.Connect)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Set some optimizations for calls queries.
	var pgQueryOptions = [...]string{
		"SET parallel_tuple_cost = 0.005;",
		"SET parallel_setup_cost = 250;",
		"SET work_mem = '8MB';",
	}
	pgConf.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		for _, o := range pgQueryOptions {
			_, err := conn.Exec(ctx, o)
			if err != nil {
				return fmt.Errorf("set option %s: %w", o, err)
			}
		}

		return RegisterDataTypes(ctx, conn)
	}

	if conf.LogQueries {
		pgConf.ConnConfig.Tracer = &tracelog.TraceLog{
			Logger:   dbLogger{},
			LogLevel: tracelog.LogLevelTrace,
		}
	}

	pool, err := pgxpool.NewWithConfig(ctx, pgConf)
	if err != nil {
		return nil, fmt.Errorf("new pool: %w", err)
	}

	db := &Postgres{
		Pool:    pool,
		Queries: New(pool),
	}

	return db, nil
}

func RegisterDataTypes(ctx context.Context, conn *pgx.Conn) error {
	dataTypeNames := []string{
		"talkgroup_tuple",
		"_talkgroup_tuple",
	}

	for _, typeName := range dataTypeNames {
		dataType, err := conn.LoadType(ctx, typeName)
		if err != nil {
			return err
		}
		conn.TypeMap().RegisterType(dataType)
	}

	return nil
}

type dBCtxKey string

const DBCtxKey dBCtxKey = "dbctx"

// FromCtx returns the database handle from the provided Context.
func FromCtx(ctx context.Context) Store {
	sv := services.Value(ctx, DBCtxKey)
	c, ok := sv.(Store)
	if !ok {
		panic("no DB in context")
	}

	return c
}

// CtxWithDB returns a Context with the provided database handle.
func CtxWithDB(ctx context.Context, conn Store) context.Context {
	return services.WithValue(ctx, DBCtxKey, conn)
}

// IsNoRows is a convenience function that returns whether a returned error is a database
// no rows error.
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

type Constraint struct {
	desc string
}

func (c Constraint) Error() error {
	return errors.New(c.desc)
}

const (
	APIKeysOwnerIDNameKey              = "api_keys_owner_id_name_key"
	AudioRefJournalCallIDBackendRefKey = "audio_ref_journal_call_id_backend_ref_key"
	SweptCallsPK                       = "swept_calls_pkey"
	TGConstraintName                   = "calls_system_talkgroup_fkey"
	SysConstraintName                  = "talkgroups_system_id_fkey"
	UsernameConstraintName             = "users_username_key"
	EmailConstraintName                = "users_email_key"
)

var Constraints = map[string]Constraint{
	APIKeysOwnerIDNameKey:              {"API key with this name already exists"},
	AudioRefJournalCallIDBackendRefKey: {"delete entry for call already exists"},
	SweptCallsPK:                       {"call already swept"},
	TGConstraintName:                   {"talkgroup already exists in system"},
	SysConstraintName:                  {"system already exists"},
	UsernameConstraintName:             {"username already taken"},
	EmailConstraintName:                {"an account with that email already exists"},
}

func ErrIsConstraintViolation(e error) *string {
	var err *pgconn.PgError
	if errors.As(e, &err) && (err.Code == "23503" || err.Code == "23505") {
		return &err.ConstraintName
	}

	return nil
}

func ConstraintViolation(e error) error {
	if cn := ErrIsConstraintViolation(e); cn != nil {
		if c, has := Constraints[*cn]; has {
			return c.Error()
		} else {
			return fmt.Errorf("constraint %s violated", *cn)
		}
	}

	return nil
}

// IsConstraintViolation is a convenience function to test whether an error is a PgError
// indicating constraint violation.
func IsConstraintViolation(err error, constraint string) bool {
	cn := ErrIsConstraintViolation(err)
	return cn != nil && *cn == constraint
}
