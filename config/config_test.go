package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any existing env vars that might interfere
	t.Setenv("HARNESS_DB_PATH", "")
	t.Setenv("DB_PATH", "")
	t.Setenv("HARNESS_LOG_LEVEL", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("HARNESS_PORT", "")
	t.Setenv("PORT", "")
	t.Setenv("HARNESS_TRANSPORT", "")
	t.Setenv("TRANSPORT", "")

	// Move to a temp directory where no .env exists
	tmpDir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DbPath != "./harness.db" {
		t.Errorf("expected DbPath './harness.db', got %q", cfg.DbPath)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("expected LogLevel INFO, got %v", cfg.LogLevel)
	}
	if cfg.Port != "9090" {
		t.Errorf("expected Port '9090', got %q", cfg.Port)
	}
	if cfg.Transport != "sse" {
		t.Errorf("expected Transport 'sse', got %q", cfg.Transport)
	}
}

func TestLoad_FromEnvVars(t *testing.T) {
	t.Setenv("HARNESS_DB_PATH", "/data/custom.db")
	t.Setenv("HARNESS_LOG_LEVEL", "debug")
	t.Setenv("HARNESS_PORT", "8080")
	t.Setenv("HARNESS_TRANSPORT", "stdio")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DbPath != "/data/custom.db" {
		t.Errorf("expected DbPath '/data/custom.db', got %q", cfg.DbPath)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("expected LogLevel DEBUG, got %v", cfg.LogLevel)
	}
	if cfg.Port != "8080" {
		t.Errorf("expected Port '8080', got %q", cfg.Port)
	}
	if cfg.Transport != "stdio" {
		t.Errorf("expected Transport 'stdio', got %q", cfg.Transport)
	}
}

func TestLoad_FromDotEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	envContent := `HARNESS_DB_PATH=/env/file.db
HARNESS_LOG_LEVEL=warn
HARNESS_PORT=7070
HARNESS_TRANSPORT=sse
`
	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		t.Fatalf("failed to write test .env file: %v", err)
	}

	cfg, err := Load(envPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DbPath != "/env/file.db" {
		t.Errorf("expected DbPath '/env/file.db', got %q", cfg.DbPath)
	}
	if cfg.LogLevel != slog.LevelWarn {
		t.Errorf("expected LogLevel WARN, got %v", cfg.LogLevel)
	}
	if cfg.Port != "7070" {
		t.Errorf("expected Port '7070', got %q", cfg.Port)
	}
	if cfg.Transport != "sse" {
		t.Errorf("expected Transport 'sse', got %q", cfg.Transport)
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	t.Setenv("HARNESS_LOG_LEVEL", "invalid_level")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid log level, got nil")
	}
}
