package db

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"ai-api-portal/backend/migrations"
)

type migrationRecord struct {
	Name      string
	AppliedAt time.Time
}

func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	if err := ensureSchemaMigrationsTable(ctx, db); err != nil {
		return err
	}

	applied, err := loadAppliedMigrations(ctx, db)
	if err != nil {
		return err
	}

	migrationFiles, err := migrations.Filenames()
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(migrationFiles)

	for _, filename := range migrationFiles {
		if !strings.HasSuffix(filename, ".sql") {
			continue
		}

		if _, exists := applied[filename]; exists {
			continue
		}

		contents, err := migrations.Read(filename)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}

		if err := applySingleMigration(ctx, db, filename, string(contents)); err != nil {
			return err
		}
	}

	return nil
}

func ensureSchemaMigrationsTable(ctx context.Context, db *sql.DB) error {
	const query = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    name TEXT PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL
);`

	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	return nil
}

func loadAppliedMigrations(ctx context.Context, db *sql.DB) (map[string]migrationRecord, error) {
	const query = `SELECT name, applied_at FROM schema_migrations;`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]migrationRecord)
	for rows.Next() {
		var record migrationRecord
		if err := rows.Scan(&record.Name, &record.AppliedAt); err != nil {
			return nil, fmt.Errorf("scan schema_migrations row: %w", err)
		}
		applied[record.Name] = record
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema_migrations rows: %w", err)
	}

	return applied, nil
}

func applySingleMigration(ctx context.Context, db *sql.DB, filename, sqlText string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for %s: %w", filename, err)
	}

	if _, err := tx.ExecContext(ctx, sqlText); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("exec migration %s: %w", filename, err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(name, applied_at) VALUES (?, ?);`, filename, time.Now().UTC()); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration %s: %w", filename, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", filename, err)
	}

	return nil
}
