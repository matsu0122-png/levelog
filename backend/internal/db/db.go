// Package db owns the Postgres connection pool and startup migrations.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// PoolOptions configures the connection pool. All values must be set
// explicitly by the caller (see config.Config) rather than defaulted here,
// so production tuning stays visible in one place.
type PoolOptions struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// Open opens a Postgres connection pool for dsn. It does not verify
// connectivity — call WaitForReady for that.
func Open(dsn string, opts PoolOptions) (*sql.DB, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	conn.SetMaxOpenConns(opts.MaxOpenConns)
	conn.SetMaxIdleConns(opts.MaxIdleConns)
	conn.SetConnMaxLifetime(opts.ConnMaxLifetime)
	return conn, nil
}

// WaitForReady pings the database, retrying until it responds or timeout
// elapses (bounded by ctx too, so a shutdown signal during startup stops
// the retry loop immediately). Useful during `docker compose up` /
// container start when the app can come up before Postgres accepts
// connections.
func WaitForReady(ctx context.Context, conn *sql.DB, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		if lastErr = conn.PingContext(ctx); lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("database not ready after %s: %w", timeout, lastErr)
		case <-ticker.C:
		}
	}
}
