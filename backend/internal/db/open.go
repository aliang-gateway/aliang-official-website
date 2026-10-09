package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, driver, dsn string) (*sql.DB, error) {
	driver = strings.ToLower(strings.TrimSpace(driver))
	dsn = strings.TrimSpace(dsn)

	switch driver {
	case "sqlite":
		if err := ensureParentDir(dsn); err != nil {
			return nil, err
		}

		// busy_timeout (and friends) are per-connection attributes; a plain
		// ExecContext only configures whichever pooled connection happens to
		// be borrowed. Thread them through the DSN so every connection in
		// the pool gets them — modernc.org/sqlite applies DSN pragmas on
		// connect.
		dsn = appendSQLiteParam(dsn, "_pragma=busy_timeout(5000)")

		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			return nil, fmt.Errorf("open sqlite: %w", err)
		}

		if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON;"); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("enable foreign keys: %w", err)
		}

		// SQLite is single-writer: without a busy timeout any concurrent
		// write (e.g. the async default-keys hook racing a registration)
		// surfaces as an immediate SQLITE_BUSY and a 500. Wait instead.
		if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000;"); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("set sqlite busy timeout: %w", err)
		}

		if err := db.PingContext(ctx); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("ping sqlite: %w", err)
		}

		return db, nil
	case "postgres":
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return nil, fmt.Errorf("open postgres: %w", err)
		}

		if err := db.PingContext(ctx); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("ping postgres: %w", err)
		}

		return db, nil
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}
}

// appendSQLiteParam adds a query parameter to a SQLite DSN whether or not it
// already carries one.
func appendSQLiteParam(dsn, param string) string {
	if strings.Contains(dsn, "?") {
		return dsn + "&" + param
	}
	return dsn + "?" + param
}

func ensureParentDir(dbPath string) error {
	parent := filepath.Dir(dbPath)
	if parent == "." || parent == "" {
		return nil
	}

	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}

	return nil
}
