package watcher

import (
	"context"
	"harness/internal/repository"
)

// CommitDiff holds metadata, modified file paths, and a bounded unified diff
// for a single git commit.
type CommitDiff struct {
	SHA         string   `json:"sha"`
	ShortSHA    string   `json:"short_sha"`
	Author      string   `json:"author"`
	CommittedAt string   `json:"committed_at"`
	Subject     string   `json:"subject"`
	Body        string   `json:"body"`
	Files       []string `json:"files"`
	Patch       string   `json:"patch"`
}

// GitInspector is the outbound port for reading git history and commit diffs.
type GitInspector interface {
	RecentCommits(ctx context.Context, repoPath, branch string, limit int) ([]string, error)
	InspectCommit(ctx context.Context, repoPath, sha string) (CommitDiff, error)
}

// CommitRecorder is the outbound port responsible for analyzing an unlogged
// commit and recording its development log entry.
type CommitRecorder interface {
	RecordCommit(ctx context.Context, project, repoPath, sha string) (*repository.DevLog, error)
}
