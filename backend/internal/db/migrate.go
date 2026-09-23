package db

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"levelog/backend/migrations"
)

// migrationLockKey is an arbitrary constant passed to pg_advisory_lock to
// serialize concurrent Migrate() calls. Its value carries no meaning beyond
// being a fixed int64 unlikely to collide with an advisory lock some other
// tool in the same database might take.
const migrationLockKey = 847822019

// Migrate applies any embedded *.sql migration files that have not yet been
// recorded in the schema_migrations table, in filename order, each inside
// its own transaction.
//
// Every step — including the initial CREATE TABLE IF NOT EXISTS — runs
// while holding a session-level Postgres advisory lock (pg_advisory_lock),
// taken on a single dedicated connection pulled from the pool and held for
// the whole call. This exists because a rolling deploy starts several app
// instances against the same database at roughly the same time (see
// docs/production-roadmap.md phase 1's "複数アプリサーバで動かない要因" #1
// and phase 13): without serialization, two instances can both pass the "IF
// NOT EXISTS" check on schema_migrations and race at the catalog level, or
// both see the same migration as unapplied and both try to INSERT its
// filename, hitting the filename primary key. Confirmed as a real,
// reproducible failure (not hypothetical) in
// internal/db/migrate_test.go's TestMigrate_ConcurrentInstancesDoNotRace
// before this lock was added. The lock is session-scoped: it releases
// automatically if the connection drops (crash, network partition), so a
// stuck lock can't outlive its holder.
func Migrate(db *sql.DB) error {
	ctx := context.Background()

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for migration lock: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	// Best-effort: if this fails, closing conn above still releases the
	// session-level lock.
	defer conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, migrationLockKey)

	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename    TEXT PRIMARY KEY,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	var filenames []string
	for _, e := range entries {
		if !e.IsDir() {
			filenames = append(filenames, e.Name())
		}
	}
	sort.Strings(filenames)

	for _, name := range filenames {
		var applied bool
		if err := conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)`, name).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			continue
		}

		sqlBytes, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, name); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	return nil
}
