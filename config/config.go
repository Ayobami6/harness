package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultWatchInterval    = time.Minute
	defaultWatchCommitLimit = 10
	maxWatchCommitLimit     = 50
	defaultWatchModel       = "gemini-2.5-flash"
)

// RepoTarget represents a single git repository watched by the background agent.
type RepoTarget struct {
	Project  string
	RepoPath string
	Branch   string
}

// Config holds application configuration settings.
type Config struct {
	DbPath           string
	LogLevel         slog.Level
	Port             string
	Transport        string
	WatchEnabled     bool
	WatchInterval    time.Duration
	WatchRepos       []RepoTarget
	WatchCommitLimit int
	WatchModel       string
	GoogleAPIKey     string
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

	watchEnabled, err := parseBool(getEnv("HARNESS_WATCH_ENABLED", getEnv("WATCH_ENABLED", "false")))
	if err != nil {
		return nil, fmt.Errorf("failed to parse watch enabled flag: %w", err)
	}

	watchInterval, err := parseDuration(getEnv("HARNESS_WATCH_INTERVAL", getEnv("WATCH_INTERVAL", "1m")), defaultWatchInterval)
	if err != nil {
		return nil, fmt.Errorf("failed to parse watch interval: %w", err)
	}

	watchCommitLimit, err := parseCommitLimit(getEnv("HARNESS_WATCH_COMMIT_LIMIT", getEnv("WATCH_COMMIT_LIMIT", "10")))
	if err != nil {
		return nil, fmt.Errorf("failed to parse watch commit limit: %w", err)
	}

	watchRepos := ParseWatchRepos(getEnv("HARNESS_WATCH_REPOS", getEnv("WATCH_REPOS", ".")))

	cfg := &Config{
		DbPath:           getEnv("HARNESS_DB_PATH", getEnv("DB_PATH", "./harness.db")),
		LogLevel:         logLevel,
		Port:             getEnv("HARNESS_PORT", getEnv("PORT", "9090")),
		Transport:        getEnv("HARNESS_TRANSPORT", getEnv("TRANSPORT", "http")),
		WatchEnabled:     watchEnabled,
		WatchInterval:    watchInterval,
		WatchRepos:       watchRepos,
		WatchCommitLimit: watchCommitLimit,
		WatchModel:       getEnv("HARNESS_WATCH_MODEL", getEnv("WATCH_MODEL", defaultWatchModel)),
		GoogleAPIKey:     getEnv("GOOGLE_API_KEY", getEnv("GEMINI_API_KEY", "")),
	}

	return cfg, nil
}

// ParseWatchRepos parses a comma-separated list of repository target specifications.
// Supported formats per entry:
//   - "project:/path/to/repo@origin/main"
//   - "project:/path/to/repo"
//   - "/path/to/repo@origin/main"
//   - "/path/to/repo" or "."
func ParseWatchRepos(raw string) []RepoTarget {
	parts := strings.Split(raw, ",")
	targets := make([]RepoTarget, 0, len(parts))

	for _, part := range parts {
		spec := strings.TrimSpace(part)
		if spec == "" {
			continue
		}

		var branch string
		if atIdx := strings.LastIndex(spec, "@"); atIdx > 0 {
			branch = strings.TrimSpace(spec[atIdx+1:])
			spec = strings.TrimSpace(spec[:atIdx])
		}

		var project, repoPath string
		// Distinguish "project:/path" from Windows drive letters like "C:\path".
		colonIdx := strings.Index(spec, ":")
		if colonIdx > 1 || (colonIdx == 1 && len(spec) > 2 && spec[2] != '\\' && spec[2] != '/') {
			project = strings.TrimSpace(spec[:colonIdx])
			repoPath = strings.TrimSpace(spec[colonIdx+1:])
		} else {
			repoPath = spec
		}

		if repoPath == "" {
			repoPath = "."
		}

		if project == "" {
			project = inferProjectName(repoPath)
		}

		targets = append(targets, RepoTarget{
			Project:  project,
			RepoPath: repoPath,
			Branch:   branch,
		})
	}

	if len(targets) == 0 {
		targets = append(targets, RepoTarget{
			Project:  inferProjectName("."),
			RepoPath: ".",
		})
	}

	return targets
}

func inferProjectName(repoPath string) string {
	abs, err := filepath.Abs(repoPath)
	if err == nil {
		base := filepath.Base(abs)
		if base != "" && base != "." && base != string(filepath.Separator) {
			return base
		}
	}
	base := filepath.Base(filepath.Clean(repoPath))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "default"
	}
	return base
}

func parseBool(raw string) (bool, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return false, nil
	}
	return strconv.ParseBool(trimmed)
}

func parseDuration(raw string, fallback time.Duration) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive, got %v", d)
	}
	return d, nil
}

func parseCommitLimit(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultWatchCommitLimit, nil
	}
	n, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return defaultWatchCommitLimit, nil
	}
	if n > maxWatchCommitLimit {
		return maxWatchCommitLimit, nil
	}
	return n, nil
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
