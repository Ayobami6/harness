package config

import (
	"context"
	"log/slog"
	"testing"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name       string
		level      slog.Level
		checkLevel slog.Level
		shouldBeOn bool
	}{
		{
			name:       "debug level allows debug",
			level:      slog.LevelDebug,
			checkLevel: slog.LevelDebug,
			shouldBeOn: true,
		},
		{
			name:       "info level disables debug",
			level:      slog.LevelInfo,
			checkLevel: slog.LevelDebug,
			shouldBeOn: false,
		},
		{
			name:       "info level allows info",
			level:      slog.LevelInfo,
			checkLevel: slog.LevelInfo,
			shouldBeOn: true,
		},
		{
			name:       "warn level disables info",
			level:      slog.LevelWarn,
			checkLevel: slog.LevelInfo,
			shouldBeOn: false,
		},
		{
			name:       "warn level allows error",
			level:      slog.LevelWarn,
			checkLevel: slog.LevelError,
			shouldBeOn: true,
		},
		{
			name:       "error level disables warn",
			level:      slog.LevelError,
			checkLevel: slog.LevelWarn,
			shouldBeOn: false,
		},
	}

	ctx := context.Background()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{
				LogLevel: tc.level,
			}

			logger := NewLogger(cfg)
			if logger == nil {
				t.Fatal("expected non-nil logger, got nil")
			}

			enabled := logger.Enabled(ctx, tc.checkLevel)
			if enabled != tc.shouldBeOn {
				t.Errorf("level %v enabled check for %v: expected %v, got %v",
					tc.level, tc.checkLevel, tc.shouldBeOn, enabled)
			}

			// Ensure logging does not panic
			logger.Log(ctx, tc.level, "test log message", "key", "value")
		})
	}
}
