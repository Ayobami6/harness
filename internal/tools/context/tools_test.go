package context

import (
	stdctx "context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"harness/config"
	"harness/db"
	"harness/internal/repository"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func setupTestDB(t *testing.T) (*sql.DB, repository.Querier) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_context.db")

	cfg := &config.Config{
		DbPath: dbPath,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := stdctx.Background()
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

CREATE TRIGGER IF NOT EXISTS dev_logs_ad AFTER DELETE ON dev_logs BEGIN
    INSERT INTO dev_logs_fts(dev_logs_fts, rowid, title, problem, solution, trade_offs, tags)
    VALUES('delete', old.id, old.title, old.problem, old.solution, old.trade_offs, old.tags);
END;

CREATE TRIGGER IF NOT EXISTS dev_logs_au AFTER UPDATE ON dev_logs BEGIN
    INSERT INTO dev_logs_fts(dev_logs_fts, rowid, title, problem, solution, trade_offs, tags)
    VALUES('delete', old.id, old.title, old.problem, old.solution, old.trade_offs, old.tags);
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

CREATE TABLE IF NOT EXISTS incidents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project TEXT NOT NULL,
    service TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    root_cause TEXT NOT NULL,
    resolution TEXT NOT NULL,
    prevention_rules TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_incidents_project ON incidents(project);

CREATE TABLE IF NOT EXISTS feature_contexts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project TEXT NOT NULL,
    feature_name TEXT NOT NULL,
    summary TEXT NOT NULL,
    critical_invariants TEXT NOT NULL,
    dependencies TEXT NOT NULL DEFAULT '[]',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project, feature_name)
);

CREATE TABLE IF NOT EXISTS code_exemplars (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    dev_log_id INTEGER REFERENCES dev_logs(id) ON DELETE SET NULL,
    language TEXT NOT NULL,
    topic TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    code_snippet TEXT NOT NULL,
    tags TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_code_exemplars_lookup ON code_exemplars(language, topic);
CREATE INDEX IF NOT EXISTS idx_code_exemplars_dev_log ON code_exemplars(dev_log_id);
`
	if _, err := conn.ExecContext(ctx, schema); err != nil {
		t.Fatalf("failed to execute schema: %v", err)
	}

	repo := repository.New(conn)
	return conn, repo
}

func makeCallRequest(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

func parseResultJSON[T any](t *testing.T, res *mcp.CallToolResult) T {
	t.Helper()
	if res.IsError {
		t.Fatalf("expected successful tool call, got error: %v", res.Content)
	}
	if len(res.Content) == 0 {
		t.Fatalf("expected tool content, got none")
	}
	text, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	var out T
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("failed to unmarshal result JSON: %v, text: %s", err, text.Text)
	}
	return out
}

func TestRegisterTools(t *testing.T) {
	conn, repo := setupTestDB(t)
	defer conn.Close()

	mod := New(repo)
	s := server.NewMCPServer("test-server", "1.0.0")

	if err := mod.RegisterTools(s); err != nil {
		t.Fatalf("RegisterTools returned unexpected error: %v", err)
	}

	expectedTools := []string{
		"save_dev_log",
		"create_dev_log",
		"get_dev_logs",
		"get_dev_logs_by_project",
		"get_dev_logs_by_service",
		"search_dev_logs_fts",
		"search_dev_logs_fts_by_project",
		"create_incident",
		"get_incidents",
		"create_architectural_profile",
		"get_architectural_profile",
		"create_feature_context",
		"get_feature_context",
		"create_code_exemplar",
		"get_code_exemplars",
	}

	for _, name := range expectedTools {
		if tool := s.GetTool(name); tool == nil {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}

func TestDevLogTools(t *testing.T) {
	conn, repo := setupTestDB(t)
	defer conn.Close()

	mod := New(repo)
	s := server.NewMCPServer("test-server", "1.0.0")
	if err := mod.RegisterTools(s); err != nil {
		t.Fatalf("RegisterTools failed: %v", err)
	}

	ctx := stdctx.Background()

	t.Run("save_dev_log successfully creates dev log", func(t *testing.T) {
		req := makeCallRequest(map[string]any{
			"project":    "harness",
			"service":    "api",
			"title":      "Added MCP Context Tools",
			"problem":    "Agents had no memory or context tools",
			"solution":   "Implemented full-text search and context tools",
			"trade_offs": "Added SQLite triggers for FTS sync",
			"tags":       "mcp,context,fts",
			"git_ref":    "HEAD",
		})

		handler := s.GetTool("save_dev_log").Handler
		res, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		log := parseResultJSON[repository.DevLog](t, res)
		if log.ID == 0 {
			t.Errorf("expected positive ID, got %d", log.ID)
		}
		if log.Project != "harness" || log.Service != "api" {
			t.Errorf("unexpected project/service: %s/%s", log.Project, log.Service)
		}
		if log.Title != "Added MCP Context Tools" {
			t.Errorf("unexpected title: %s", log.Title)
		}
	})

	t.Run("save_dev_log missing required arguments", func(t *testing.T) {
		req := makeCallRequest(map[string]any{
			"project": "harness",
			// missing title, problem, solution
		})

		handler := s.GetTool("save_dev_log").Handler
		res, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("handler should not return go error, got %v", err)
		}
		if !res.IsError {
			t.Error("expected tool error result for missing fields")
		}
	})

	t.Run("create_dev_log alias with tags as array", func(t *testing.T) {
		req := makeCallRequest(map[string]any{
			"project":  "payment-svc",
			"service":  "ledger",
			"title":    "Double-entry ledger invariant",
			"problem":  "Concurrency race condition during balance updates",
			"solution": "Applied row-level locking with SELECT FOR UPDATE",
			"tags":     []any{"concurrency", "postgres", "ledger"},
		})

		handler := s.GetTool("create_dev_log").Handler
		res, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		log := parseResultJSON[repository.DevLog](t, res)
		if log.Tags != "concurrency,postgres,ledger" {
			t.Errorf("expected joined tags, got %q", log.Tags)
		}
	})

	t.Run("get_dev_logs across all, by project, and by service", func(t *testing.T) {
		// All
		handler := s.GetTool("get_dev_logs").Handler
		res, err := handler(ctx, makeCallRequest(map[string]any{"limit": 10}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		logs := parseResultJSON[[]repository.DevLog](t, res)
		if len(logs) != 2 {
			t.Fatalf("expected 2 logs, got %d", len(logs))
		}

		// By Project
		resProj, err := handler(ctx, makeCallRequest(map[string]any{"project": "payment-svc"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		projLogs := parseResultJSON[[]repository.DevLog](t, resProj)
		if len(projLogs) != 1 || projLogs[0].Project != "payment-svc" {
			t.Fatalf("expected 1 log for payment-svc, got %v", projLogs)
		}

		// By Service
		resSvc, err := handler(ctx, makeCallRequest(map[string]any{"project": "harness", "service": "api"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		svcLogs := parseResultJSON[[]repository.DevLog](t, resSvc)
		if len(svcLogs) != 1 || svcLogs[0].Service != "api" {
			t.Fatalf("expected 1 log for harness/api, got %v", svcLogs)
		}
	})

	t.Run("get_dev_logs_by_project and get_dev_logs_by_service standalone", func(t *testing.T) {
		handlerProj := s.GetTool("get_dev_logs_by_project").Handler
		res, err := handlerProj(ctx, makeCallRequest(map[string]any{"project": "harness"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		logs := parseResultJSON[[]repository.DevLog](t, res)
		if len(logs) != 1 {
			t.Errorf("expected 1 log, got %d", len(logs))
		}

		// Missing required project
		resErr, _ := handlerProj(ctx, makeCallRequest(map[string]any{}))
		if !resErr.IsError {
			t.Error("expected error result when project is missing")
		}

		handlerSvc := s.GetTool("get_dev_logs_by_service").Handler
		resSvc, err := handlerSvc(ctx, makeCallRequest(map[string]any{"project": "harness", "service": "api"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		logsSvc := parseResultJSON[[]repository.DevLog](t, resSvc)
		if len(logsSvc) != 1 {
			t.Errorf("expected 1 log, got %d", len(logsSvc))
		}
	})

	t.Run("search_dev_logs_fts global and by project", func(t *testing.T) {
		handlerSearch := s.GetTool("search_dev_logs_fts").Handler

		// Global search matching "concurrency"
		res, err := handlerSearch(ctx, makeCallRequest(map[string]any{"query": "concurrency"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rows := parseResultJSON[[]repository.SearchDevLogsFTSRow](t, res)
		if len(rows) != 1 {
			t.Fatalf("expected 1 match for 'concurrency', got %d", len(rows))
		}
		if rows[0].Project != "payment-svc" {
			t.Errorf("expected match from payment-svc, got %s", rows[0].Project)
		}

		// Search with project filter matching "harness"
		resProj, err := handlerSearch(ctx, makeCallRequest(map[string]any{"query": "Context", "project": "harness"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rowsProj := parseResultJSON[[]repository.SearchDevLogsFTSByProjectRow](t, resProj)
		if len(rowsProj) != 1 {
			t.Fatalf("expected 1 match for 'Context' in harness, got %d", len(rowsProj))
		}

		// Search with project filter with non-matching project
		resNone, err := handlerSearch(ctx, makeCallRequest(map[string]any{"query": "Context", "project": "unknown-proj"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rowsNone := parseResultJSON[[]repository.SearchDevLogsFTSByProjectRow](t, resNone)
		if len(rowsNone) != 0 {
			t.Fatalf("expected 0 matches, got %d", len(rowsNone))
		}

		// search_dev_logs_fts_by_project standalone
		handlerSearchProj := s.GetTool("search_dev_logs_fts_by_project").Handler
		resExact, err := handlerSearchProj(ctx, makeCallRequest(map[string]any{"query": "ledger", "project": "payment-svc"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rowsExact := parseResultJSON[[]repository.SearchDevLogsFTSByProjectRow](t, resExact)
		if len(rowsExact) != 1 {
			t.Fatalf("expected 1 match, got %d", len(rowsExact))
		}
	})
}

func TestIncidentTools(t *testing.T) {
	conn, repo := setupTestDB(t)
	defer conn.Close()

	mod := New(repo)
	s := server.NewMCPServer("test-server", "1.0.0")
	if err := mod.RegisterTools(s); err != nil {
		t.Fatalf("RegisterTools failed: %v", err)
	}

	ctx := stdctx.Background()

	t.Run("create_incident and get_incidents", func(t *testing.T) {
		createHandler := s.GetTool("create_incident").Handler
		req := makeCallRequest(map[string]any{
			"project":          "billing-engine",
			"service":          "stripe-webhook",
			"title":            "Duplicate Charge on Network Retry",
			"root_cause":       "Webhook consumer was missing idempotency keys",
			"resolution":       "Added distributed lock and deduplication table",
			"prevention_rules": "Always enforce idempotency_key on payment mutations",
		})

		res, err := createHandler(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		incident := parseResultJSON[repository.Incident](t, res)
		if incident.ID == 0 {
			t.Errorf("expected non-zero ID, got %d", incident.ID)
		}
		if incident.Title != "Duplicate Charge on Network Retry" {
			t.Errorf("unexpected title: %s", incident.Title)
		}

		// Missing field
		errRes, _ := createHandler(ctx, makeCallRequest(map[string]any{"project": "billing-engine"}))
		if !errRes.IsError {
			t.Error("expected error result for missing fields")
		}

		// Get all incidents
		getHandler := s.GetTool("get_incidents").Handler
		getRes, err := getHandler(ctx, makeCallRequest(map[string]any{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		list := parseResultJSON[[]repository.Incident](t, getRes)
		if len(list) != 1 {
			t.Fatalf("expected 1 incident, got %d", len(list))
		}

		// Get by project and service
		svcRes, err := getHandler(ctx, makeCallRequest(map[string]any{"project": "billing-engine", "service": "stripe-webhook"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		svcList := parseResultJSON[[]repository.Incident](t, svcRes)
		if len(svcList) != 1 {
			t.Errorf("expected 1 incident, got %d", len(svcList))
		}
	})
}

func TestArchitecturalProfileTools(t *testing.T) {
	conn, repo := setupTestDB(t)
	defer conn.Close()

	mod := New(repo)
	s := server.NewMCPServer("test-server", "1.0.0")
	if err := mod.RegisterTools(s); err != nil {
		t.Fatalf("RegisterTools failed: %v", err)
	}

	ctx := stdctx.Background()

	t.Run("create_architectural_profile upserts and get retrieves", func(t *testing.T) {
		createHandler := s.GetTool("create_architectural_profile").Handler
		getHandler := s.GetTool("get_architectural_profile").Handler

		// Initially not found
		notRes, err := getHandler(ctx, makeCallRequest(map[string]any{"project": "core-harness"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !notRes.IsError {
			t.Error("expected not found error")
		}

		// Create profile
		req := makeCallRequest(map[string]any{
			"project":            "core-harness",
			"architecture_style": "Hexagonal Architecture",
			"conventions":        []string{"Ports & Adapters", "Pure domain models"},
			"core_stack":         []string{"Go", "SQLite", "mcp-go"},
			"rules":              []string{"Never import db in domain", "Always wrap errors"},
		})

		createRes, err := createHandler(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		profile := parseResultJSON[repository.ArchitecturalProfile](t, createRes)
		if profile.Project != "core-harness" {
			t.Errorf("unexpected project: %s", profile.Project)
		}

		// Fetch profile
		getRes, err := getHandler(ctx, makeCallRequest(map[string]any{"project": "core-harness"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fetched := parseResultJSON[repository.ArchitecturalProfile](t, getRes)
		if fetched.ArchitectureStyle != "Hexagonal Architecture" {
			t.Errorf("unexpected style: %s", fetched.ArchitectureStyle)
		}

		// Upsert (update)
		updateReq := makeCallRequest(map[string]any{
			"project":            "core-harness",
			"architecture_style": "Hexagonal DDD",
		})
		updateRes, err := createHandler(ctx, updateReq)
		if err != nil {
			t.Fatalf("unexpected update error: %v", err)
		}
		updated := parseResultJSON[repository.ArchitecturalProfile](t, updateRes)
		if updated.ArchitectureStyle != "Hexagonal DDD" {
			t.Errorf("expected updated style, got %s", updated.ArchitectureStyle)
		}
	})
}

func TestFeatureContextTools(t *testing.T) {
	conn, repo := setupTestDB(t)
	defer conn.Close()

	mod := New(repo)
	s := server.NewMCPServer("test-server", "1.0.0")
	if err := mod.RegisterTools(s); err != nil {
		t.Fatalf("RegisterTools failed: %v", err)
	}

	ctx := stdctx.Background()

	t.Run("create_feature_context and get_feature_context", func(t *testing.T) {
		createHandler := s.GetTool("create_feature_context").Handler
		getHandler := s.GetTool("get_feature_context").Handler

		req := makeCallRequest(map[string]any{
			"project":             "harness",
			"feature_name":        "memory_mcp",
			"summary":             "Long-term memory context for AI coding agents",
			"critical_invariants": []string{"FTS index always in sync with dev_logs"},
			"dependencies":        []string{"modernc.org/sqlite", "mark3labs/mcp-go"},
		})

		res, err := createHandler(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fc := parseResultJSON[repository.FeatureContext](t, res)
		if fc.FeatureName != "memory_mcp" {
			t.Errorf("unexpected feature name: %s", fc.FeatureName)
		}

		// Get specific feature context
		getExact, err := getHandler(ctx, makeCallRequest(map[string]any{
			"project":      "harness",
			"feature_name": "memory_mcp",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		exact := parseResultJSON[repository.FeatureContext](t, getExact)
		if exact.Summary != "Long-term memory context for AI coding agents" {
			t.Errorf("unexpected summary: %s", exact.Summary)
		}

		// List for project
		getList, err := getHandler(ctx, makeCallRequest(map[string]any{"project": "harness"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		list := parseResultJSON[[]repository.FeatureContext](t, getList)
		if len(list) != 1 {
			t.Errorf("expected 1 feature context, got %d", len(list))
		}
	})
}

func TestCodeExemplarTools(t *testing.T) {
	conn, repo := setupTestDB(t)
	defer conn.Close()

	mod := New(repo)
	s := server.NewMCPServer("test-server", "1.0.0")
	if err := mod.RegisterTools(s); err != nil {
		t.Fatalf("RegisterTools failed: %v", err)
	}

	ctx := stdctx.Background()

	t.Run("create_code_exemplar and get_code_exemplars", func(t *testing.T) {
		createHandler := s.GetTool("create_code_exemplar").Handler
		getHandler := s.GetTool("get_code_exemplars").Handler

		req := makeCallRequest(map[string]any{
			"language":     "go",
			"topic":        "error_handling",
			"title":        "Explicit error wrapping",
			"code_snippet": "if err != nil { return fmt.Errorf(\"failed to load: %w\", err) }",
			"description":  "Wrap errors with context for observability",
			"tags":         "error,wrapping,idiomatic",
		})

		res, err := createHandler(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		exemplar := parseResultJSON[repository.CodeExemplar](t, res)
		if exemplar.ID == 0 {
			t.Errorf("expected positive ID, got %d", exemplar.ID)
		}
		if exemplar.Language != "go" || exemplar.Topic != "error_handling" {
			t.Errorf("unexpected language/topic: %s/%s", exemplar.Language, exemplar.Topic)
		}

		// Filter by language & topic
		filterRes, err := getHandler(ctx, makeCallRequest(map[string]any{
			"language": "go",
			"topic":    "error_handling",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		list := parseResultJSON[[]repository.CodeExemplar](t, filterRes)
		if len(list) != 1 {
			t.Fatalf("expected 1 exemplar, got %d", len(list))
		}

		// Filter by language only
		langRes, err := getHandler(ctx, makeCallRequest(map[string]any{"language": "go"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		langList := parseResultJSON[[]repository.CodeExemplar](t, langRes)
		if len(langList) != 1 {
			t.Fatalf("expected 1 exemplar, got %d", len(langList))
		}

		// Filter all
		allRes, err := getHandler(ctx, makeCallRequest(map[string]any{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		allList := parseResultJSON[[]repository.CodeExemplar](t, allRes)
		if len(allList) != 1 {
			t.Fatalf("expected 1 exemplar, got %d", len(allList))
		}
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("extractPagination bounds", func(t *testing.T) {
		reqHigh := makeCallRequest(map[string]any{"limit": 500, "offset": -10})
		limit, offset := extractPagination(reqHigh)
		if limit != 100 {
			t.Errorf("expected max limit 100, got %d", limit)
		}
		if offset != 0 {
			t.Errorf("expected non-negative offset 0, got %d", offset)
		}

		reqLow := makeCallRequest(map[string]any{"limit": 0})
		limitLow, _ := extractPagination(reqLow)
		if limitLow != 20 {
			t.Errorf("expected default limit 20, got %d", limitLow)
		}
	})

	t.Run("parseTags with various shapes", func(t *testing.T) {
		reqStr := makeCallRequest(map[string]any{"tags": "one,two"})
		if parseTags(reqStr, "tags") != "one,two" {
			t.Errorf("unexpected tags: %s", parseTags(reqStr, "tags"))
		}

		reqSlice := makeCallRequest(map[string]any{"tags": []string{"alpha", "beta"}})
		if parseTags(reqSlice, "tags") != "alpha,beta" {
			t.Errorf("unexpected tags: %s", parseTags(reqSlice, "tags"))
		}

		reqEmpty := makeCallRequest(map[string]any{})
		if parseTags(reqEmpty, "tags") != "" {
			t.Errorf("expected empty tags, got %s", parseTags(reqEmpty, "tags"))
		}
	})

	t.Run("parseJSONOrString fallbacks and formatting", func(t *testing.T) {
		reqEmpty := makeCallRequest(map[string]any{})
		if parseJSONOrString(reqEmpty, "rules", "[]") != "[]" {
			t.Errorf("expected fallback, got %s", parseJSONOrString(reqEmpty, "rules", "[]"))
		}

		reqSlice := makeCallRequest(map[string]any{"rules": []string{"rule1"}})
		if parseJSONOrString(reqSlice, "rules", "[]") != `["rule1"]` {
			t.Errorf("expected JSON array string, got %s", parseJSONOrString(reqSlice, "rules", "[]"))
		}
	})
}
