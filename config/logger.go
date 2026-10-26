package config

import (
	"log/slog"
	"os"
)

type LoggerConfig struct {
	Level string `mapstructure:"level" json:"level" yaml:"level"`
}

// NewLogger initiates a new slog.logger
func NewLogger(cfg Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	// Create a new logger with the specified options
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	return logger
}
