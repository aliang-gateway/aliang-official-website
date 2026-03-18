package db

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func TestApplyMigrationsCreatesRequiredTables(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbFile := filepath.Join(t.TempDir(), "test.db")

	database, err := Open(ctx, dbFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := ApplyMigrations(ctx, database); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}

	if err := ApplyMigrations(ctx, database); err != nil {
		t.Fatalf("ApplyMigrations() second run error = %v", err)
	}

	tables := []string{
		"schema_migrations",
		"users",
		"sessions",
		"api_keys",
		"tiers",
		"service_items",
		"tier_default_items",
		"subscriptions",
		"subscription_overrides",
		"unit_prices",
		"usage_records",
	}

	for _, table := range tables {
		table := table
		t.Run(fmt.Sprintf("table_%s_exists", table), func(t *testing.T) {
			t.Parallel()

			var count int
			err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?;`, table).Scan(&count)
			if err != nil {
				t.Fatalf("table existence query failed for %s: %v", table, err)
			}
			if count != 1 {
				t.Fatalf("expected table %s to exist", table)
			}
		})
	}

	t.Run("users_table_has_role_column", func(t *testing.T) {
		var count int
		err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'role';`).Scan(&count)
		if err != nil {
			t.Fatalf("role column existence query failed: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected users.role column to exist")
		}
	})

	t.Run("users_table_has_password_hash_column", func(t *testing.T) {
		var count int
		err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'password_hash';`).Scan(&count)
		if err != nil {
			t.Fatalf("password_hash column existence query failed: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected users.password_hash column to exist")
		}
	})
}
