package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// busy_timeout is a per-connection SQLite attribute. A plain ExecContext
// only touches whichever pooled connection happens to be borrowed, leaving
// every other pooled connection at the default 0 (immediate SQLITE_BUSY).
// Open must thread the pragma through the DSN so every connection gets it.
func TestOpenSqliteBusyTimeoutAppliesToEveryConnection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database, err := Open(ctx, "sqlite", filepath.Join(t.TempDir(), "busy.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	database.SetMaxOpenConns(3)

	conns := make([]*sql.Conn, 0, 2)
	for i := 0; i < 2; i++ {
		conn, err := database.Conn(ctx)
		if err != nil {
			t.Fatalf("acquire connection %d: %v", i+1, err)
		}
		conns = append(conns, conn)
		t.Cleanup(func() { _ = conn.Close() })
	}

	for i, conn := range conns {
		var timeout int
		if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout;").Scan(&timeout); err != nil {
			t.Fatalf("query busy_timeout on connection %d: %v", i+1, err)
		}
		if timeout != 5000 {
			t.Fatalf("connection %d busy_timeout = %d, want 5000 (pragma must apply pool-wide)", i+1, timeout)
		}
	}
}
