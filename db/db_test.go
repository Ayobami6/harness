package db

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"harness/config"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestConnectDB_Success(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.Config{
		DbPath: dbPath,
	}

	ctx := context.Background()
	conn, err := ConnectDB(ctx, cfg, newTestLogger())
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	defer conn.Close()

	if conn == nil {
		t.Fatal("expected non-nil *sql.DB connection")
	}

	// Verify connection pool limits
	stats := conn.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Errorf("expected MaxOpenConnections 1, got %d", stats.MaxOpenConnections)
	}

	// Verify basic query execution
	var val int
	if err := conn.QueryRowContext(ctx, "SELECT 1").Scan(&val); err != nil {
		t.Fatalf("failed to query database: %v", err)
	}
	if val != 1 {
		t.Errorf("expected query result 1, got %d", val)
	}
}

func TestConnectDB_Pragmas(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "pragmas.db")

	cfg := &config.Config{
		DbPath: dbPath,
	}

	ctx := context.Background()
	conn, err := ConnectDB(ctx, cfg, newTestLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer conn.Close()

	// Verify foreign_keys pragma is enabled
	var foreignKeys int
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys;").Scan(&foreignKeys); err != nil {
		t.Fatalf("failed to check foreign_keys pragma: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("expected foreign_keys = 1, got %d", foreignKeys)
	}

	// Verify busy_timeout pragma
	var busyTimeout int
	if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout;").Scan(&busyTimeout); err != nil {
		t.Fatalf("failed to check busy_timeout pragma: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("expected busy_timeout = 5000, got %d", busyTimeout)
	}
}

func TestConnectDB_EmptyDbPath(t *testing.T) {
	cfg := &config.Config{
		DbPath: "",
	}

	conn, err := ConnectDB(context.Background(), cfg, newTestLogger())
	if err == nil {
		if conn != nil {
			_ = conn.Close()
		}
		t.Fatal("expected error for empty DbPath, got nil")
	}

	if conn != nil {
		t.Errorf("expected nil *sql.DB on error, got %v", conn)
	}

	if !strings.Contains(err.Error(), "database path is empty") {
		t.Errorf("expected error message to contain 'database path is empty', got %q", err.Error())
	}
}

func TestConnectDB_CanceledContext(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "canceled.db")

	cfg := &config.Config{
		DbPath: dbPath,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	conn, err := ConnectDB(ctx, cfg, newTestLogger())
	if err == nil {
		if conn != nil {
			_ = conn.Close()
		}
		t.Fatal("expected error with canceled context, got nil")
	}

	if conn != nil {
		t.Errorf("expected nil *sql.DB when ping fails, got %v", conn)
	}
}

func TestConnectDB_InMemory(t *testing.T) {
	cfg := &config.Config{
		DbPath: ":memory:",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := ConnectDB(ctx, cfg, newTestLogger())
	if err != nil {
		t.Fatalf("unexpected error connecting to in-memory db: %v", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT);"); err != nil {
		t.Fatalf("failed to create table in in-memory db: %v", err)
	}
}
