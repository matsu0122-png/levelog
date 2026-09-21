// Package db owns the Postgres connection pool and startup migrations.
package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open opens a Postgres connection pool for dsn and verifies connectivity.
func Open(dsn string) (*sql.DB, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	conn.SetMaxOpenConns(20)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(30 * time.Minute)
	return conn, nil
}

// WaitForReady pings the database, retrying until it responds or the
// deadline elapses. Useful during `docker compose up` when the app
// container can start before Postgres is accepting connections.
func WaitForReady(conn *sql.DB, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if lastErr = conn.Ping(); lastErr == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("database not ready after %s: %w", timeout, lastErr)
}
