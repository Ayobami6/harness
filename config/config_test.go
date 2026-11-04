package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	t.Setenv("HARNESS_WATCH_ENABLED", "")
	t.Setenv("WATCH_ENABLED", "")
	t.Setenv("HARNESS_WATCH_INTERVAL", "")
	t.Setenv("WATCH_INTERVAL", "")
	t.Setenv("HARNESS_WATCH_REPOS", "")
	t.Setenv("WATCH_REPOS", "")
	t.Setenv("HARNESS_WATCH_COMMIT_LIMIT", "")
	t.Setenv("WATCH_COMMIT_LIMIT", "")
	t.Setenv("HARNESS_WATCH_MODEL", "")
	t.Setenv("WATCH_MODEL", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")

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
	if cfg.Transport != "http" {
		t.Errorf("expected Transport 'http', got %q", cfg.Transport)
	}
	if cfg.WatchEnabled {
		t.Errorf("expected WatchEnabled false by default")
	}
	if cfg.WatchInterval != time.Minute {
		t.Errorf("expected WatchInterval 1m, got %v", cfg.WatchInterval)
	}
	if cfg.WatchCommitLimit != 10 {
		t.Errorf("expected WatchCommitLimit 10, got %d", cfg.WatchCommitLimit)
	}
	if cfg.WatchModel != "gemini-2.5-flash" {
		t.Errorf("expected WatchModel 'gemini-2.5-flash', got %q", cfg.WatchModel)
	}
	if len(cfg.WatchRepos) != 1 || cfg.WatchRepos[0].RepoPath != "." {
		t.Errorf("expected single default watch repo '.', got %+v", cfg.WatchRepos)
	}
}

func TestLoad_FromEnvVars(t *testing.T) {
	t.Setenv("HARNESS_DB_PATH", "/data/custom.db")
	t.Setenv("HARNESS_LOG_LEVEL", "debug")
	t.Setenv("HARNESS_PORT", "8080")
	t.Setenv("HARNESS_TRANSPORT", "stdio")
	t.Setenv("HARNESS_WATCH_ENABLED", "true")
	t.Setenv("HARNESS_WATCH_INTERVAL", "45s")
	t.Setenv("HARNESS_WATCH_REPOS", "payments:/srv/payments@origin/main, /srv/ledger")
	t.Setenv("HARNESS_WATCH_COMMIT_LIMIT", "25")
	t.Setenv("HARNESS_WATCH_MODEL", "gemini-2.5-pro")
	t.Setenv("GEMINI_API_KEY", "test-gemini-key")

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
	if !cfg.WatchEnabled {
		t.Errorf("expected WatchEnabled true")
	}
	if cfg.WatchInterval != 45*time.Second {
		t.Errorf("expected WatchInterval 45s, got %v", cfg.WatchInterval)
	}
	if cfg.WatchCommitLimit != 25 {
		t.Errorf("expected WatchCommitLimit 25, got %d", cfg.WatchCommitLimit)
	}
	if cfg.WatchModel != "gemini-2.5-pro" {
		t.Errorf("expected WatchModel 'gemini-2.5-pro', got %q", cfg.WatchModel)
	}
	if cfg.GoogleAPIKey != "test-gemini-key" {
		t.Errorf("expected GoogleAPIKey 'test-gemini-key', got %q", cfg.GoogleAPIKey)
	}
	if len(cfg.WatchRepos) != 2 {
		t.Fatalf("expected 2 watch repos, got %d", len(cfg.WatchRepos))
	}
	if cfg.WatchRepos[0].Project != "payments" || cfg.WatchRepos[0].RepoPath != "/srv/payments" || cfg.WatchRepos[0].Branch != "origin/main" {
		t.Errorf("unexpected first repo target: %+v", cfg.WatchRepos[0])
	}
	if cfg.WatchRepos[1].Project != "ledger" || cfg.WatchRepos[1].RepoPath != "/srv/ledger" || cfg.WatchRepos[1].Branch != "" {
		t.Errorf("unexpected second repo target: %+v", cfg.WatchRepos[1])
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

func TestLoad_InvalidWatchSettings(t *testing.T) {
	t.Run("invalid watch enabled bool", func(t *testing.T) {
		t.Setenv("HARNESS_WATCH_ENABLED", "not_a_bool")
		if _, err := Load(); err == nil {
			t.Fatal("expected error for invalid HARNESS_WATCH_ENABLED")
		}
	})

	t.Run("invalid watch interval", func(t *testing.T) {
		t.Setenv("HARNESS_WATCH_INTERVAL", "-5s")
		if _, err := Load(); err == nil {
			t.Fatal("expected error for negative HARNESS_WATCH_INTERVAL")
		}
	})

	t.Run("invalid watch commit limit", func(t *testing.T) {
		t.Setenv("HARNESS_WATCH_COMMIT_LIMIT", "abc")
		if _, err := Load(); err == nil {
			t.Fatal("expected error for non-numeric HARNESS_WATCH_COMMIT_LIMIT")
		}
	})

	t.Run("commit limit bounds clamping", func(t *testing.T) {
		t.Setenv("HARNESS_WATCH_COMMIT_LIMIT", "500")
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.WatchCommitLimit != 50 {
			t.Errorf("expected clamped limit 50, got %d", cfg.WatchCommitLimit)
		}
	})
}
