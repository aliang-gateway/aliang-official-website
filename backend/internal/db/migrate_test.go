package db

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"ai-api-portal/backend/migrations"
)

func TestApplyMigrationsCreatesRequiredTables(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbFile := filepath.Join(t.TempDir(), "test.db")

	database, err := Open(ctx, "sqlite", dbFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := ApplyMigrations(ctx, database, "sqlite"); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}

	if err := ApplyMigrations(ctx, database, "sqlite"); err != nil {
		t.Fatalf("ApplyMigrations() second run error = %v", err)
	}

	tables := []string{
		"als_schema_migrations",
		"als_users",
		"als_sessions",
		"als_sub2api_auth_tokens",
		"als_api_keys",
		"als_tiers",
		"als_service_items",
		"als_tier_default_items",
		"als_subscriptions",
		"als_subscription_overrides",
		"als_unit_prices",
		"als_usage_records",
		"als_tier_group_bindings",
		"als_fulfillment_jobs",
		"als_fulfillment_events",
		"als_payment_records",
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
		err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('als_users') WHERE name = 'role';`).Scan(&count)
		if err != nil {
			t.Fatalf("role column existence query failed: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected als_users.role column to exist")
		}
	})

	t.Run("users_table_has_password_hash_column", func(t *testing.T) {
		var count int
		err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('als_users') WHERE name = 'password_hash';`).Scan(&count)
		if err != nil {
			t.Fatalf("password_hash column existence query failed: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected als_users.password_hash column to exist")
		}
	})
}

func TestApplyMigrationsRejectsUnsupportedDialect(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbFile := filepath.Join(t.TempDir(), "test.db")

	database, err := Open(ctx, "sqlite", dbFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	err = ApplyMigrations(ctx, database, "mysql")
	if err == nil {
		t.Fatalf("expected unsupported dialect error")
	}
	if !strings.Contains(err.Error(), "unsupported database dialect") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMigrationFilesExistForBothDialects(t *testing.T) {
	t.Parallel()

	sqliteFiles, err := migrations.Filenames("sqlite")
	if err != nil {
		t.Fatalf("migrations.Filenames(sqlite) error = %v", err)
	}
	postgresFiles, err := migrations.Filenames("postgres")
	if err != nil {
		t.Fatalf("migrations.Filenames(postgres) error = %v", err)
	}

	if len(sqliteFiles) == 0 {
		t.Fatalf("expected sqlite migration files")
	}
	if len(postgresFiles) == 0 {
		t.Fatalf("expected postgres migration files")
	}

	sqliteByName := make(map[string]string, len(sqliteFiles))
	for _, file := range sqliteFiles {
		sqliteByName[file.Name] = file.Path
	}

	for _, file := range postgresFiles {
		if _, ok := sqliteByName[file.Name]; !ok {
			t.Fatalf("postgres migration %q missing sqlite counterpart", file.Name)
		}
	}

	for name := range sqliteByName {
		found := false
		for _, file := range postgresFiles {
			if file.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("sqlite migration %q missing postgres counterpart", name)
		}
	}
}
