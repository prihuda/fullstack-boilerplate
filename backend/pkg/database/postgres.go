package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// PostgresConfig holds PostgreSQL connection configuration.
type PostgresConfig struct {
	DatabaseURL  string
	MaxConns     int32
	MinConns     int32
	MaxConnIdle  time.Duration
	MaxConnLife  time.Duration
	HealthCheck  time.Duration
}

// DefaultPostgresConfig returns a config with sensible defaults.
func DefaultPostgresConfig(databaseURL string) PostgresConfig {
	return PostgresConfig{
		DatabaseURL: databaseURL,
		MaxConns:    20,
		MinConns:    5,
		MaxConnIdle: 30 * time.Minute,
		MaxConnLife: 2 * time.Hour,
		HealthCheck: 1 * time.Minute,
	}
}

// DB holds the Bun ORM wrapper backed by a pgx connection pool.
type DB struct {
	pool  *pgxpool.Pool
	BunDB *bun.DB
}

// NewDB creates a new DB with a pgx pool and Bun ORM wrapper.
func NewDB(ctx context.Context, cfg PostgresConfig) (*DB, error) {
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database url: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdle
	poolCfg.MaxConnLifetime = cfg.MaxConnLife
	poolCfg.HealthCheckPeriod = cfg.HealthCheck

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	// Wrap pgx pool with Bun ORM
	sqldb := stdlib.OpenDBFromPool(pool)
	bunDB := bun.NewDB(sqldb, pgdialect.New())

	return &DB{
		pool:  pool,
		BunDB: bunDB,
	}, nil
}

// Ping checks database connectivity.
func (db *DB) Ping(ctx context.Context) error {
	return db.BunDB.PingContext(ctx)
}

// Close closes the database connections.
func (db *DB) Close() error {
	if err := db.BunDB.Close(); err != nil {
		db.pool.Close()
		return err
	}
	db.pool.Close()
	return nil
}
