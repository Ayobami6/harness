package watcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMaxPatchBytes = 24 * 1024 // 24 KiB cap to protect LLM context window
	defaultFetchTimeout  = 15 * time.Second
)

var (
	shaPattern = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)
	refPattern = regexp.MustCompile(`^[a-zA-Z0-9._/@~^-]+$`)
)

// ExecGitInspector implements GitInspector using the local git CLI.
type ExecGitInspector struct {
	logger        *slog.Logger
	maxPatchBytes int
	fetchTimeout  time.Duration
}

// NewExecGitInspector creates a new ExecGitInspector with safe defaults.
func NewExecGitInspector(logger *slog.Logger) *ExecGitInspector {
	if logger == nil {
		logger = slog.Default()
	}
	return &ExecGitInspector{
		logger:        logger,
		maxPatchBytes: defaultMaxPatchBytes,
		fetchTimeout:  defaultFetchTimeout,
	}
}

// RecentCommits fetches remote updates (best-effort) and returns up to limit
// commit SHAs in chronological order (oldest to newest).
func (g *ExecGitInspector) RecentCommits(ctx context.Context, repoPath, branch string, limit int) ([]string, error) {
	cleanPath := filepath.Clean(repoPath)
	if limit <= 0 {
		limit = 10
	}

	// Best-effort git fetch so origin/<branch> reflects remote pushes.
	fetchCtx, cancel := context.WithTimeout(ctx, g.fetchTimeout)
	if _, err := runGit(fetchCtx, cleanPath, "fetch", "--quiet"); err != nil {
		g.logger.Debug("git fetch skipped or failed (continuing with local refs)", "repo", cleanPath, "error", err)
	}
	cancel()

	targetRef, err := g.resolveRef(ctx, cleanPath, branch)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve git ref for %s: %w", cleanPath, err)
	}

	out, err := runGit(
		ctx,
		cleanPath,
		"log",
		targetRef,
		"-n",
		strconv.Itoa(limit),
		"--reverse",
		"--no-merges",
		"--format=%H",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list recent commits on %s: %w", targetRef, err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	shas := make([]string, 0, len(lines))
	for _, line := range lines {
		sha := strings.TrimSpace(line)
		if sha != "" && shaPattern.MatchString(sha) {
			shas = append(shas, sha)
		}
	}
	return shas, nil
}

// InspectCommit extracts commit metadata, changed files, and a size-bounded patch.
func (g *ExecGitInspector) InspectCommit(ctx context.Context, repoPath, sha string) (CommitDiff, error) {
	cleanPath := filepath.Clean(repoPath)
	trimmedSHA := strings.TrimSpace(sha)
	if !shaPattern.MatchString(trimmedSHA) {
		return CommitDiff{}, fmt.Errorf("invalid commit SHA %q", sha)
	}

	// Extract structured header fields separated by newline:
	// line 1: full SHA
	// line 2: short SHA
	// line 3: author name <email>
	// line 4: ISO 8601 commit date
	// line 5: subject
	// remainder: body
	metaOut, err := runGit(
		ctx,
		cleanPath,
		"show",
		"-s",
		"--format=%H%n%h%n%an <%ae>%n%cI%n%s%n%b",
		trimmedSHA,
	)
	if err != nil {
		return CommitDiff{}, fmt.Errorf("failed to inspect commit metadata for %s: %w", trimmedSHA, err)
	}

	metaLines := strings.SplitN(metaOut, "\n", 6)
	if len(metaLines) < 5 {
		return CommitDiff{}, fmt.Errorf("unexpected git show metadata format for %s", trimmedSHA)
	}

	var body string
	if len(metaLines) == 6 {
		body = strings.TrimSpace(metaLines[5])
	}

	filesOut, err := runGit(
		ctx,
		cleanPath,
		"diff-tree",
		"--root",
		"--no-commit-id",
		"--name-only",
		"-r",
		trimmedSHA,
	)
	if err != nil {
		return CommitDiff{}, fmt.Errorf("failed to list changed files for %s: %w", trimmedSHA, err)
	}

	var files []string
	for _, f := range strings.Split(strings.TrimSpace(filesOut), "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			files = append(files, f)
		}
	}

	patchOut, err := runGit(
		ctx,
		cleanPath,
		"show",
		"--format=",
		"--stat",
		"--patch",
		trimmedSHA,
	)
	if err != nil {
		return CommitDiff{}, fmt.Errorf("failed to read commit patch for %s: %w", trimmedSHA, err)
	}

	maxBytes := g.maxPatchBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxPatchBytes
	}
	if len(patchOut) > maxBytes {
		patchOut = patchOut[:maxBytes] + "\n... [diff truncated]"
	}

	return CommitDiff{
		SHA:         strings.TrimSpace(metaLines[0]),
		ShortSHA:    strings.TrimSpace(metaLines[1]),
		Author:      strings.TrimSpace(metaLines[2]),
		CommittedAt: strings.TrimSpace(metaLines[3]),
		Subject:     strings.TrimSpace(metaLines[4]),
		Body:        body,
		Files:       files,
		Patch:       strings.TrimSpace(patchOut),
	}, nil
}

func (g *ExecGitInspector) resolveRef(ctx context.Context, repoPath, branch string) (string, error) {
	trimmed := strings.TrimSpace(branch)
	if trimmed != "" {
		if strings.HasPrefix(trimmed, "-") || !refPattern.MatchString(trimmed) {
			return "", fmt.Errorf("invalid branch reference %q", branch)
		}
		if refExists(ctx, repoPath, trimmed) {
			return trimmed, nil
		}
		if !strings.HasPrefix(trimmed, "origin/") && refExists(ctx, repoPath, "origin/"+trimmed) {
			return "origin/" + trimmed, nil
		}
	}

	// Try upstream tracking branch of current HEAD, then origin/main, origin/master, and finally HEAD.
	candidates := []string{"@{u}", "origin/main", "origin/master", "HEAD"}
	for _, candidate := range candidates {
		if refExists(ctx, repoPath, candidate) {
			return candidate, nil
		}
	}

	return "", errors.New("no valid git reference found (repository may have no commits)")
}

func refExists(ctx context.Context, repoPath, ref string) bool {
	_, err := runGit(ctx, repoPath, "rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

func runGit(ctx context.Context, repoPath string, args ...string) (string, error) {
	fullArgs := make([]string, 0, len(args)+2)
	fullArgs = append(fullArgs, "-C", repoPath)
	fullArgs = append(fullArgs, args...)

	// #nosec G204 -- git arguments are validated against strict allowlists and fixed subcommands
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, errMsg)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}
