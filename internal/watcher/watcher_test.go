package watcher

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"harness/config"
	"harness/db"
	"harness/internal/repository"
)

func setupTestDB(t *testing.T) (*sql.DB, repository.Querier) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_watcher.db")

	cfg := &config.Config{
		DbPath: dbPath,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	conn, err := db.ConnectDB(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	schema := `
CREATE TABLE IF NOT EXISTS dev_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project TEXT NOT NULL,
    service TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    problem TEXT NOT NULL,
    solution TEXT NOT NULL,
    trade_offs TEXT NOT NULL DEFAULT '',
    tags TEXT NOT NULL DEFAULT '',
    git_ref TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_dev_logs_project ON dev_logs(project);
CREATE INDEX IF NOT EXISTS idx_dev_logs_service ON dev_logs(project, service);
CREATE INDEX IF NOT EXISTS idx_dev_logs_created ON dev_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_dev_logs_project_git_ref ON dev_logs(project, git_ref);

CREATE VIRTUAL TABLE IF NOT EXISTS dev_logs_fts USING fts5(
    title,
    problem,
    solution,
    trade_offs,
    tags,
    content='dev_logs',
    content_rowid='id'
);

CREATE TRIGGER IF NOT EXISTS dev_logs_ai AFTER INSERT ON dev_logs BEGIN
    INSERT INTO dev_logs_fts(rowid, title, problem, solution, trade_offs, tags)
    VALUES (new.id, new.title, new.problem, new.solution, new.trade_offs, new.tags);
END;

CREATE TABLE IF NOT EXISTS architectural_profiles (
    project TEXT PRIMARY KEY,
    architecture_style TEXT NOT NULL,
    conventions TEXT NOT NULL,
    core_stack TEXT NOT NULL,
    rules TEXT NOT NULL DEFAULT '[]',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`
	if _, err := conn.ExecContext(ctx, schema); err != nil {
		_ = conn.Close()
		t.Fatalf("failed to create test schema: %v", err)
	}

	return conn, repository.New(conn)
}

func initTempGitRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	runGitCmd(t, repoDir, "init", "-b", "main")
	runGitCmd(t, repoDir, "config", "user.name", "Test Engineer")
	runGitCmd(t, repoDir, "config", "user.email", "test@example.com")
	return repoDir
}

func commitFile(t *testing.T, repoDir, relPath, content, message string) string {
	t.Helper()

	fullPath := filepath.Join(repoDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0750); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	runGitCmd(t, repoDir, "add", relPath)
	runGitCmd(t, repoDir, "commit", "-m", message)
	return strings.TrimSpace(runGitCmd(t, repoDir, "rev-parse", "HEAD"))
}

func runGitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()

	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\noutput: %s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

func TestExecGitInspector_RecentAndInspect(t *testing.T) {
	repoDir := initTempGitRepo(t)
	sha1 := commitFile(t, repoDir, "internal/service.go", "package internal\n\nfunc One() {}\n", "feat(service): add One\n\nDetailed body explaining One.")
	sha2 := commitFile(t, repoDir, "internal/service.go", "package internal\n\nfunc One() {}\nfunc Two() {}\n", "feat(service): add Two")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	inspector := NewExecGitInspector(logger)
	ctx := context.Background()

	shas, err := inspector.RecentCommits(ctx, repoDir, "", 10)
	if err != nil {
		t.Fatalf("RecentCommits failed: %v", err)
	}
	if len(shas) != 2 {
		t.Fatalf("expected 2 commits, got %d (%v)", len(shas), shas)
	}
	if shas[0] != sha1 || shas[1] != sha2 {
		t.Errorf("expected chronological order [%s, %s], got %v", sha1, sha2, shas)
	}

	diff, err := inspector.InspectCommit(ctx, repoDir, sha1)
	if err != nil {
		t.Fatalf("InspectCommit failed: %v", err)
	}
	if diff.SHA != sha1 {
		t.Errorf("expected SHA %s, got %s", sha1, diff.SHA)
	}
	if diff.Subject != "feat(service): add One" {
		t.Errorf("unexpected subject %q", diff.Subject)
	}
	if !strings.Contains(diff.Body, "Detailed body explaining One.") {
		t.Errorf("unexpected body %q", diff.Body)
	}
	if len(diff.Files) != 1 || diff.Files[0] != "internal/service.go" {
		t.Errorf("unexpected files: %v", diff.Files)
	}
	if !strings.Contains(diff.Patch, "+func One() {}") {
		t.Errorf("expected patch to contain '+func One() {}', got:\n%s", diff.Patch)
	}

	t.Run("truncates oversized patch", func(t *testing.T) {
		smallInspector := NewExecGitInspector(logger)
		smallInspector.maxPatchBytes = 32
		truncated, err := smallInspector.InspectCommit(ctx, repoDir, sha1)
		if err != nil {
			t.Fatalf("InspectCommit failed: %v", err)
		}
		if !strings.HasSuffix(truncated.Patch, "... [diff truncated]") {
			t.Errorf("expected truncated suffix, got: %s", truncated.Patch)
		}
	})

	t.Run("rejects invalid SHA", func(t *testing.T) {
		if _, err := inspector.InspectCommit(ctx, repoDir, "--help"); err == nil {
			t.Fatal("expected error for invalid SHA")
		}
	})

	t.Run("rejects invalid branch flag injection", func(t *testing.T) {
		if _, err := inspector.RecentCommits(ctx, repoDir, "--upload-pack=evil", 5); err == nil {
			t.Fatal("expected error for invalid branch reference")
		}
	})
}

type fakeCommitRecorder struct {
	mu       sync.Mutex
	repo     repository.Querier
	recorded []string
}

func (f *fakeCommitRecorder) RecordCommit(ctx context.Context, project, _ string, sha string) (*repository.DevLog, error) {
	f.mu.Lock()
	f.recorded = append(f.recorded, sha)
	f.mu.Unlock()

	created, err := f.repo.CreateDevLog(ctx, repository.CreateDevLogParams{
		Project:   project,
		Service:   "watcher",
		Title:     "Auto log for " + sha[:7],
		Problem:   "Simulated problem",
		Solution:  "Simulated solution",
		TradeOffs: "None",
		Tags:      "auto,git",
		GitRef:    sha,
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func TestWorker_ReconcileAndIdempotency(t *testing.T) {
	conn, repo := setupTestDB(t)
	defer conn.Close()

	repoDir := initTempGitRepo(t)
	sha1 := commitFile(t, repoDir, "a.go", "package main\n", "feat: commit 1")
	sha2 := commitFile(t, repoDir, "b.go", "package main\n", "feat: commit 2")
	sha3 := commitFile(t, repoDir, "c.go", "package main\n", "feat: commit 3")

	ctx := context.Background()

	// Pre-seed sha1 using only its 7-char short SHA to verify short-SHA prefix idempotency.
	_, err := repo.CreateDevLog(ctx, repository.CreateDevLogParams{
		Project:  "harness",
		Service:  "core",
		Title:    "Manual log for commit 1",
		Problem:  "Initial setup",
		Solution: "Added a.go",
		GitRef:   sha1[:7],
	})
	if err != nil {
		t.Fatalf("failed to seed sha1 dev log: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	inspector := NewExecGitInspector(logger)
	recorder := &fakeCommitRecorder{repo: repo}

	worker := NewWorker(WorkerConfig{
		Repo:        repo,
		Git:         inspector,
		Recorder:    recorder,
		Logger:      logger,
		Targets:     []config.RepoTarget{{Project: "harness", RepoPath: repoDir, Branch: "main"}},
		Interval:    50 * time.Millisecond,
		CommitLimit: 10,
	})

	// First pass: sha1 should be skipped (matched via 7-char short SHA), sha2 and sha3 recorded.
	worker.Reconcile(ctx)

	if len(recorder.recorded) != 2 {
		t.Fatalf("expected 2 commits recorded on first pass, got %d (%v)", len(recorder.recorded), recorder.recorded)
	}
	if recorder.recorded[0] != sha2 || recorder.recorded[1] != sha3 {
		t.Errorf("expected [%s, %s], got %v", sha2, sha3, recorder.recorded)
	}

	// Second pass: all 3 commits now exist in dev_logs; 0 new recordings should happen.
	worker.Reconcile(ctx)
	if len(recorder.recorded) != 2 {
		t.Errorf("expected no duplicate recordings on second pass, got %d", len(recorder.recorded))
	}

	// Verify Start(ctx) loop stops cleanly when context is canceled.
	runCtx, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
	defer cancel()
	worker.Start(runCtx)
}

func TestCommitToolSet_Handlers(t *testing.T) {
	conn, repo := setupTestDB(t)
	defer conn.Close()

	repoDir := initTempGitRepo(t)
	sha := commitFile(t, repoDir, "watcher.go", "package watcher\n", "feat(watcher): add ticker")

	ctx := context.Background()
	_, err := repo.UpsertArchitecturalProfile(ctx, repository.UpsertArchitecturalProfileParams{
		Project:           "harness",
		ArchitectureStyle: "Hexagonal Architecture",
		Conventions:       `["ports and adapters"]`,
		CoreStack:         `["Go", "SQLite", "MCP"]`,
		Rules:             `["no CGO"]`,
	})
	if err != nil {
		t.Fatalf("failed to seed architectural profile: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	inspector := NewExecGitInspector(logger)
	ts := newCommitToolSet(repo, inspector, "harness", repoDir, sha)

	tools, err := ts.buildADKTools()
	if err != nil {
		t.Fatalf("buildADKTools failed: %v", err)
	}
	if len(tools) != 4 {
		t.Fatalf("expected 4 ADK tools, got %d", len(tools))
	}

	diff, err := ts.handleGetCommitDiff(ctx, getCommitDiffArgs{})
	if err != nil {
		t.Fatalf("handleGetCommitDiff failed: %v", err)
	}
	if diff.SHA != sha {
		t.Errorf("expected SHA %s, got %s", sha, diff.SHA)
	}

	profileRes, err := ts.handleGetArchProfile(ctx, getArchProfileArgs{})
	if err != nil {
		t.Fatalf("handleGetArchProfile failed: %v", err)
	}
	if !profileRes.Found || profileRes.Profile == nil || profileRes.Profile.ArchitectureStyle != "Hexagonal Architecture" {
		t.Errorf("unexpected profile result: %+v", profileRes)
	}

	saveRes, err := ts.handleSaveDevLog(ctx, saveDevLogArgs{
		Service:   "watcher",
		Title:     "Add background git watcher",
		Problem:   "Unlogged git pushes",
		Solution:  "Ticker + ADK agent",
		TradeOffs: "Polling interval latency",
		Tags:      "git,watcher,adk",
	})
	if err != nil {
		t.Fatalf("handleSaveDevLog failed: %v", err)
	}
	if !saveRes.Saved || saveRes.DevLog.GitRef != sha {
		t.Errorf("unexpected save result: %+v", saveRes)
	}

	// Calling handleSaveDevLog a second time in the same session should be idempotent.
	saveRes2, err := ts.handleSaveDevLog(ctx, saveDevLogArgs{
		Title:    "Duplicate call",
		Problem:  "Duplicate",
		Solution: "Duplicate",
	})
	if err != nil {
		t.Fatalf("second handleSaveDevLog failed: %v", err)
	}
	if saveRes2.DevLog.ID != saveRes.DevLog.ID {
		t.Errorf("expected idempotent DevLog ID %d, got %d", saveRes.DevLog.ID, saveRes2.DevLog.ID)
	}

	// Verify FTS search finds the newly saved log.
	searchRes, err := ts.handleSearchDevLogs(ctx, searchDevLogsArgs{Query: "watcher", Limit: 5})
	if err != nil {
		t.Fatalf("handleSearchDevLogs failed: %v", err)
	}
	if len(searchRes.Logs) != 1 {
		t.Errorf("expected 1 FTS search hit, got %d", len(searchRes.Logs))
	}
}

func TestNewADKCommitRecorder_Validation(t *testing.T) {
	conn, repo := setupTestDB(t)
	defer conn.Close()
	inspector := NewExecGitInspector(nil)

	if _, err := NewADKCommitRecorder(ADKRecorderConfig{Git: inspector}); err == nil {
		t.Fatal("expected error when Repo is nil")
	}
	if _, err := NewADKCommitRecorder(ADKRecorderConfig{Repo: repo}); err == nil {
		t.Fatal("expected error when Git is nil")
	}

	rec, err := NewADKCommitRecorder(ADKRecorderConfig{
		Repo: repo,
		Git:  inspector,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := rec.RecordCommit(context.Background(), "harness", ".", "a24118c"); err == nil {
		t.Fatal("expected error when APIKey is empty")
	}
}
