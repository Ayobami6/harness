package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds application configuration settings.
type Config struct {
	DbPath    string
	LogLevel  slog.Level
	Port      string
	Transport string
}

// Load reads configuration from environment variables, optionally loading
// from .env file(s) first via godotenv. If no filenames are supplied, godotenv
// looks for a ".env" file in the current directory; if that file does not exist,
// the missing file is ignored and system environment variables are used.
func Load(filenames ...string) (*Config, error) {
	if err := godotenv.Load(filenames...); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to load environment file: %w", err)
	}

	logLevel, err := parseLogLevel(getEnv("HARNESS_LOG_LEVEL", getEnv("LOG_LEVEL", "info")))
	if err != nil {
		return nil, fmt.Errorf("failed to parse log level: %w", err)
	}

	cfg := &Config{
		DbPath:    getEnv("HARNESS_DB_PATH", getEnv("DB_PATH", "./harness.db")),
		LogLevel:  logLevel,
		Port:      getEnv("HARNESS_PORT", getEnv("PORT", "9090")),
		Transport: getEnv("HARNESS_TRANSPORT", getEnv("TRANSPORT", "http")),
	}

	return cfg, nil
}

// getEnv retrieves the environment variable for the given key, returning fallback if empty.
func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return val
	}
	return fallback
}

// parseLogLevel converts a string representation of a log level into slog.Level.
func parseLogLevel(lvl string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(lvl)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		var level slog.Level
		if err := level.UnmarshalText([]byte(lvl)); err != nil {
			return slog.LevelInfo, fmt.Errorf("invalid log level %q: %w", lvl, err)
		}
		return level, nil
	}
}
