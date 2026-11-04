package watcher

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"harness/internal/repository"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/genai"
)

const (
	adkAppName = "harness_git_watcher"
	adkUserID  = "harness_background_worker"

	adkSystemInstruction = `You are the Harness Background Engineering Memory Agent.
Your responsibility is to inspect a newly pushed git commit, understand its architectural context, and record a concise, high-signal development log entry in Harness.

Follow this exact workflow:
1. Call get_commit_diff to inspect the commit message, modified files, and unified diff patch.
2. Call get_architectural_profile to check if the project has defined architecture conventions or stack rules.
3. Optionally call search_dev_logs with 1-3 keywords if you need historical context on the component being modified.
4. Call save_dev_log ONCE with:
   - title: A clear, descriptive title of the change (max 100 chars).
   - service: The primary package, module, or service affected (e.g. "watcher", "mcp", "db", "config").
   - problem: Why this commit was necessary (the bug, missing capability, or architectural gap).
   - solution: How the commit solved it (key types, functions, queries, or patterns changed).
   - trade_offs: Notable design trade-offs, constraints, or operational considerations.
   - tags: Comma-separated lowercase tags (e.g. "git,watcher,adk,sqlite").`
)

// ADKRecorderConfig configures the Google ADK commit recorder.
type ADKRecorderConfig struct {
	Repo      repository.Querier
	Git       GitInspector
	ModelName string
	APIKey    string
	Logger    *slog.Logger
}

// ADKCommitRecorder implements CommitRecorder using Google ADK for Go (v2).
type ADKCommitRecorder struct {
	repo      repository.Querier
	git       GitInspector
	modelName string
	apiKey    string
	logger    *slog.Logger
}

// NewADKCommitRecorder creates a new ADKCommitRecorder.
func NewADKCommitRecorder(cfg ADKRecorderConfig) (*ADKCommitRecorder, error) {
	if cfg.Repo == nil {
		return nil, errors.New("repository querier is required")
	}
	if cfg.Git == nil {
		return nil, errors.New("git inspector is required")
	}
	modelName := strings.TrimSpace(cfg.ModelName)
	if modelName == "" {
		modelName = "gemini-3.5-flash-lite"
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &ADKCommitRecorder{
		repo:      cfg.Repo,
		git:       cfg.Git,
		modelName: modelName,
		apiKey:    strings.TrimSpace(cfg.APIKey),
		logger:    logger,
	}, nil
}

// RecordCommit runs a scoped Google ADK agent session for the given commit SHA
// and returns the persisted DevLog.
func (r *ADKCommitRecorder) RecordCommit(ctx context.Context, project, repoPath, sha string) (*repository.DevLog, error) {
	if r.apiKey == "" {
		return nil, errors.New("missing GOOGLE_API_KEY or GEMINI_API_KEY for ADK commit recorder")
	}

	model, err := gemini.NewModel(ctx, r.modelName, &genai.ClientConfig{
		APIKey: r.apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ADK Gemini model %q: %w", r.modelName, err)
	}

	ts := newCommitToolSet(r.repo, r.git, project, repoPath, sha)
	tools, err := ts.buildADKTools()
	if err != nil {
		return nil, fmt.Errorf("failed to build ADK function tools: %w", err)
	}

	commitAgent, err := llmagent.New(llmagent.Config{
		Name:        "harness_commit_log_agent",
		Model:       model,
		Description: "Analyzes a git commit diff and records a structured development log in Harness.",
		Instruction: adkSystemInstruction,
		Tools:       tools,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ADK llmagent: %w", err)
	}

	sessionService := session.InMemoryService()
	sessResp, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName: adkAppName,
		UserID:  adkUserID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ADK session: %w", err)
	}

	adkRunner, err := runner.New(runner.Config{
		AppName:        adkAppName,
		Agent:          commitAgent,
		SessionService: sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ADK runner: %w", err)
	}

	prompt := fmt.Sprintf(
		"Analyze git commit %s in project %q (repository path %q) and record its development log if it does not already exist.",
		sha,
		project,
		repoPath,
	)
	userMsg := &genai.Content{
		Role: string(genai.RoleUser),
		Parts: []*genai.Part{
			genai.NewPartFromText(prompt),
		},
	}

	for event, runErr := range adkRunner.Run(ctx, adkUserID, sessResp.Session.ID(), userMsg, agent.RunConfig{
		StreamingMode: agent.StreamingModeNone,
	}) {
		if runErr != nil {
			return nil, fmt.Errorf("ADK agent run failed for commit %s: %w", sha, runErr)
		}
		if event != nil && event.Content != nil {
			for _, part := range event.Content.Parts {
				if part != nil && strings.TrimSpace(part.Text) != "" {
					r.logger.Debug("ADK agent response", "sha", sha, "text", strings.TrimSpace(part.Text))
				}
			}
		}
	}

	saved := ts.savedDevLog()
	if saved == nil {
		return nil, fmt.Errorf("ADK agent completed without invoking save_dev_log for commit %s", sha)
	}
	return saved, nil
}

// --- Scoped ADK Function Tools ---

type getCommitDiffArgs struct {
	SHA string `json:"sha,omitempty" jsonschema:"Optional commit SHA to inspect (defaults to the active commit SHA)"`
}

type getArchProfileArgs struct {
	Project string `json:"project,omitempty" jsonschema:"Optional project identifier (defaults to the active project)"`
}

type getArchProfileResult struct {
	Found   bool                             `json:"found"`
	Profile *repository.ArchitecturalProfile `json:"profile,omitempty"`
}

type searchDevLogsArgs struct {
	Query string `json:"query" jsonschema:"Full-text search keywords to find related development logs"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of results to return (default 5)"`
}

type searchDevLogsResult struct {
	Logs []repository.SearchDevLogsFTSByProjectRow `json:"logs"`
}

type saveDevLogArgs struct {
	Service   string `json:"service,omitempty" jsonschema:"Service, package, or module name modified in the commit"`
	Title     string `json:"title" jsonschema:"Concise title summarizing the commit change"`
	Problem   string `json:"problem" jsonschema:"Problem or requirement that motivated this commit"`
	Solution  string `json:"solution" jsonschema:"Technical solution implemented in the commit"`
	TradeOffs string `json:"trade_offs,omitempty" jsonschema:"Architectural trade-offs, constraints, or operational notes"`
	Tags      string `json:"tags,omitempty" jsonschema:"Comma-separated tags (e.g. 'git,watcher,adk')"`
}

type saveDevLogResult struct {
	Saved  bool              `json:"saved"`
	DevLog repository.DevLog `json:"dev_log"`
}

type commitToolSet struct {
	repo     repository.Querier
	git      GitInspector
	project  string
	repoPath string
	sha      string

	mu    sync.Mutex
	saved *repository.DevLog
}

func newCommitToolSet(repo repository.Querier, git GitInspector, project, repoPath, sha string) *commitToolSet {
	return &commitToolSet{
		repo:     repo,
		git:      git,
		project:  project,
		repoPath: repoPath,
		sha:      sha,
	}
}

func (ts *commitToolSet) savedDevLog() *repository.DevLog {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.saved
}

func (ts *commitToolSet) handleGetCommitDiff(ctx context.Context, _ getCommitDiffArgs) (CommitDiff, error) {
	return ts.git.InspectCommit(ctx, ts.repoPath, ts.sha)
}

func (ts *commitToolSet) handleGetArchProfile(ctx context.Context, _ getArchProfileArgs) (getArchProfileResult, error) {
	profile, err := ts.repo.GetArchitecturalProfileByProject(ctx, ts.project)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return getArchProfileResult{Found: false}, nil
		}
		return getArchProfileResult{}, fmt.Errorf("failed to get architectural profile: %w", err)
	}
	return getArchProfileResult{
		Found:   true,
		Profile: &profile,
	}, nil
}

func (ts *commitToolSet) handleSearchDevLogs(ctx context.Context, args searchDevLogsArgs) (searchDevLogsResult, error) {
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return searchDevLogsResult{Logs: []repository.SearchDevLogsFTSByProjectRow{}}, nil
	}
	limit := args.Limit
	if limit <= 0 || limit > 20 {
		limit = 5
	}

	rows, err := ts.repo.SearchDevLogsFTSByProject(ctx, repository.SearchDevLogsFTSByProjectParams{
		Query:   query,
		Project: ts.project,
		Limit:   int64(limit),
		Offset:  0,
	})
	if err != nil {
		// Gracefully degrade if the FTS query contains special characters.
		return searchDevLogsResult{Logs: []repository.SearchDevLogsFTSByProjectRow{}}, nil
	}
	if rows == nil {
		rows = []repository.SearchDevLogsFTSByProjectRow{}
	}
	return searchDevLogsResult{Logs: rows}, nil
}

func (ts *commitToolSet) handleSaveDevLog(ctx context.Context, args saveDevLogArgs) (saveDevLogResult, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Idempotency guard within the same agent session.
	if ts.saved != nil {
		return saveDevLogResult{Saved: true, DevLog: *ts.saved}, nil
	}

	title := strings.TrimSpace(args.Title)
	problem := strings.TrimSpace(args.Problem)
	solution := strings.TrimSpace(args.Solution)
	if title == "" || problem == "" || solution == "" {
		return saveDevLogResult{}, errors.New("title, problem, and solution are required")
	}

	shortSHA := ts.sha
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}

	// Database-level idempotency check before insert.
	exists, err := ts.repo.ExistsDevLogByGitRef(ctx, repository.ExistsDevLogByGitRefParams{
		Project:  ts.project,
		FullSha:  ts.sha,
		ShortSha: shortSHA,
	})
	if err != nil {
		return saveDevLogResult{}, fmt.Errorf("failed to check existing dev log: %w", err)
	}
	if exists != 0 {
		existing, getErr := ts.repo.GetDevLogByGitRef(ctx, repository.GetDevLogByGitRefParams{
			Project: ts.project,
			GitRef:  ts.sha,
		})
		if getErr == nil {
			ts.saved = &existing
			return saveDevLogResult{Saved: true, DevLog: existing}, nil
		}
	}

	created, err := ts.repo.CreateDevLog(ctx, repository.CreateDevLogParams{
		Project:   ts.project,
		Service:   strings.TrimSpace(args.Service),
		Title:     title,
		Problem:   problem,
		Solution:  solution,
		TradeOffs: strings.TrimSpace(args.TradeOffs),
		Tags:      strings.TrimSpace(args.Tags),
		GitRef:    ts.sha,
	})
	if err != nil {
		return saveDevLogResult{}, fmt.Errorf("failed to persist dev log: %w", err)
	}

	ts.saved = &created
	return saveDevLogResult{
		Saved:  true,
		DevLog: created,
	}, nil
}

func (ts *commitToolSet) buildADKTools() ([]tool.Tool, error) {
	diffTool, err := functiontool.New(
		functiontool.Config{
			Name:        "get_commit_diff",
			Description: "Retrieves the commit message, author, changed files, and unified diff patch for the target git commit.",
		},
		func(ctx agent.Context, args getCommitDiffArgs) (CommitDiff, error) {
			return ts.handleGetCommitDiff(ctx, args)
		},
	)
	if err != nil {
		return nil, err
	}

	profileTool, err := functiontool.New(
		functiontool.Config{
			Name:        "get_architectural_profile",
			Description: "Retrieves the project's architectural profile (style, conventions, core stack, and rules) if one exists.",
		},
		func(ctx agent.Context, args getArchProfileArgs) (getArchProfileResult, error) {
			return ts.handleGetArchProfile(ctx, args)
		},
	)
	if err != nil {
		return nil, err
	}

	searchTool, err := functiontool.New(
		functiontool.Config{
			Name:        "search_dev_logs",
			Description: "Searches existing project dev logs using FTS5 BM25 ranking for historical context.",
		},
		func(ctx agent.Context, args searchDevLogsArgs) (searchDevLogsResult, error) {
			return ts.handleSearchDevLogs(ctx, args)
		},
	)
	if err != nil {
		return nil, err
	}

	saveTool, err := functiontool.New(
		functiontool.Config{
			Name:        "save_dev_log",
			Description: "Saves the synthesized development log entry for the active commit into Harness.",
		},
		func(ctx agent.Context, args saveDevLogArgs) (saveDevLogResult, error) {
			return ts.handleSaveDevLog(ctx, args)
		},
	)
	if err != nil {
		return nil, err
	}

	return []tool.Tool{diffTool, profileTool, searchTool, saveTool}, nil
}
