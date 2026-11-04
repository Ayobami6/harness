package watcher

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"harness/config"
	"harness/internal/repository"
)

const defaultPerCommitTimeout = 45 * time.Second

// Worker polls configured git repositories on a ticker and invokes the
// CommitRecorder for any commit whose SHA is not yet recorded in dev_logs.
type Worker struct {
	repo             repository.Querier
	git              GitInspector
	recorder         CommitRecorder
	logger           *slog.Logger
	targets          []config.RepoTarget
	interval         time.Duration
	commitLimit      int
	perCommitTimeout time.Duration
}

// WorkerConfig holds dependencies and settings for constructing a Worker.
type WorkerConfig struct {
	Repo             repository.Querier
	Git              GitInspector
	Recorder         CommitRecorder
	Logger           *slog.Logger
	Targets          []config.RepoTarget
	Interval         time.Duration
	CommitLimit      int
	PerCommitTimeout time.Duration
}

// NewWorker initializes a new background git watcher Worker.
func NewWorker(cfg WorkerConfig) *Worker {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	interval := cfg.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	commitLimit := cfg.CommitLimit
	if commitLimit <= 0 {
		commitLimit = 10
	}
	perCommitTimeout := cfg.PerCommitTimeout
	if perCommitTimeout <= 0 {
		perCommitTimeout = defaultPerCommitTimeout
	}

	return &Worker{
		repo:             cfg.Repo,
		git:              cfg.Git,
		recorder:         cfg.Recorder,
		logger:           logger,
		targets:          cfg.Targets,
		interval:         interval,
		commitLimit:      commitLimit,
		perCommitTimeout: perCommitTimeout,
	}
}

// Start runs an immediate reconciliation pass and then continues polling on
// every ticker interval until ctx is canceled.
func (w *Worker) Start(ctx context.Context) {
	w.logger.Info("Starting background git diff watcher",
		"interval", w.interval.String(),
		"commit_limit", w.commitLimit,
		"repo_count", len(w.targets),
	)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.Reconcile(ctx)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Stopping background git diff watcher")
			return
		case <-ticker.C:
			w.Reconcile(ctx)
		}
	}
}

// Reconcile inspects all configured repository targets once and records dev
// logs for any unlogged commits.
func (w *Worker) Reconcile(ctx context.Context) {
	for _, target := range w.targets {
		if ctx.Err() != nil {
			return
		}
		w.reconcileTarget(ctx, target)
	}
}

func (w *Worker) reconcileTarget(ctx context.Context, target config.RepoTarget) {
	shas, err := w.git.RecentCommits(ctx, target.RepoPath, target.Branch, w.commitLimit)
	if err != nil {
		w.logger.Warn("Failed to list recent commits for watched repository",
			"project", target.Project,
			"repo_path", target.RepoPath,
			"branch", target.Branch,
			"error", err,
		)
		return
	}

	for _, sha := range shas {
		if ctx.Err() != nil {
			return
		}

		fullSHA := strings.TrimSpace(sha)
		shortSHA := fullSHA
		if len(shortSHA) > 7 {
			shortSHA = shortSHA[:7]
		}

		exists, err := w.repo.ExistsDevLogByGitRef(ctx, repository.ExistsDevLogByGitRefParams{
			Project:  target.Project,
			FullSha:  fullSHA,
			ShortSha: shortSHA,
		})
		if err != nil {
			w.logger.Error("Failed to check existing dev log for commit",
				"project", target.Project,
				"sha", shortSHA,
				"error", err,
			)
			continue
		}
		if exists != 0 {
			continue
		}

		w.logger.Info("Unlogged commit detected; invoking ADK agent",
			"project", target.Project,
			"sha", shortSHA,
		)

		stepCtx, cancel := context.WithTimeout(ctx, w.perCommitTimeout)
		devLog, err := w.recorder.RecordCommit(stepCtx, target.Project, target.RepoPath, fullSHA)
		cancel()
		if err != nil {
			w.logger.Warn("Failed to record dev log for commit",
				"project", target.Project,
				"sha", shortSHA,
				"error", err,
			)
			continue
		}

		if devLog != nil {
			w.logger.Info("Recorded dev log from git commit",
				"project", target.Project,
				"sha", shortSHA,
				"dev_log_id", devLog.ID,
				"title", devLog.Title,
			)
		}
	}
}
