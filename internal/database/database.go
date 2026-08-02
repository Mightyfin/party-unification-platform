package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Database, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	cfg.MaxConns, cfg.MinConns, cfg.MaxConnLifetime = 20, 2, 30*time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	d := &Database{pool: pool}
	if err = d.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return d, nil
}
func (d *Database) Ping(ctx context.Context) error            { return d.pool.Ping(ctx) }
func (d *Database) Close()                                    { d.pool.Close() }
func (d *Database) Begin(ctx context.Context) (pgx.Tx, error) { return d.pool.Begin(ctx) }
func (d *Database) QueryRow(ctx context.Context, q string, args ...any) pgx.Row {
	return d.pool.QueryRow(ctx, q, args...)
}
func (d *Database) Exec(ctx context.Context, q string, args ...any) (pgconn.CommandTag, error) {
	return d.pool.Exec(ctx, q, args...)
}
