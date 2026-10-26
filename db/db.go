package db

import (
	"context"
	"database/sql"
	"fmt"
	"harness/config"
	"log/slog"
	"time"

	_ "modernc.org/sqlite"
)

// ConnectDB connects the sqlite database for harness
func ConnectDB(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*sql.DB, error) {
	if cfg.DbPath == "" {
		logger.Error("Database path is empty. Please set HARNESS_DB_PATH or DB_PATH environment variable.‼️😡")
		return nil, fmt.Errorf("database path is empty 🙄")
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)", cfg.DbPath)

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w 😪", err)
	}

	// SQLite operates best with a single writer to avoid SQLITE_BUSY deadlocks
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(time.Hour)

	// Verify connectivity
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := conn.PingContext(pingCtx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Connected to SQLite database successfully. 🎉")

	return conn, nil
}
