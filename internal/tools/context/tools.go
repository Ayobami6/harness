package context

import (
	stdctx "context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"harness/internal/repository"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Module implements the mcp.ToolModule interface to provide context-retrieval
// and memory persistence tools for the Harness platform.
type Module struct {
	repo repository.Querier
}

// New creates a new context tools Module backed by the provided repository querier.
func New(repo repository.Querier) *Module {
	return &Module{repo: repo}
}

// RegisterTools registers all context-related tools on the provided MCP server.
func (m *Module) RegisterTools(s *server.MCPServer) error {
	tools := []struct {
		tool    mcp.Tool
		handler server.ToolHandlerFunc
	}{
		// Dev Log Tools
		{
			tool: mcp.NewTool("save_dev_log",
				mcp.WithDescription("Save a development log entry recording problem, solution, trade-offs, and git ref"),
				mcp.WithString("project", mcp.Required(), mcp.Description("Project identifier")),
				mcp.WithString("service", mcp.Description("Service or component within the project")),
				mcp.WithString("title", mcp.Required(), mcp.Description("Title of the development log entry")),
				mcp.WithString("problem", mcp.Required(), mcp.Description("Problem description and symptoms")),
				mcp.WithString("solution", mcp.Required(), mcp.Description("Implemented solution and rationale")),
				mcp.WithString("trade_offs", mcp.Description("Trade-offs, alternative approaches considered, or residual risks")),
				mcp.WithString("tags", mcp.Description("Comma-separated tags (e.g. 'auth,jwt,tokens')")),
				mcp.WithString("git_ref", mcp.Description("Git commit hash, branch, or PR reference")),
			),
			handler: m.handleSaveDevLog,
		},
		{
			tool: mcp.NewTool("create_dev_log",
				mcp.WithDescription("Alias for save_dev_log: creates a new development log entry"),
				mcp.WithString("project", mcp.Required(), mcp.Description("Project identifier")),
				mcp.WithString("service", mcp.Description("Service or component within the project")),
				mcp.WithString("title", mcp.Required(), mcp.Description("Title of the development log entry")),
				mcp.WithString("problem", mcp.Required(), mcp.Description("Problem description and symptoms")),
				mcp.WithString("solution", mcp.Required(), mcp.Description("Implemented solution and rationale")),
				mcp.WithString("trade_offs", mcp.Description("Trade-offs, alternative approaches considered, or residual risks")),
				mcp.WithString("tags", mcp.Description("Comma-separated tags (e.g. 'auth,jwt,tokens')")),
				mcp.WithString("git_ref", mcp.Description("Git commit hash, branch, or PR reference")),
			),
			handler: m.handleSaveDevLog,
		},
		{
			tool: mcp.NewTool("get_dev_logs",
				mcp.WithDescription("Retrieve development logs, optionally filtered by project and service, ordered from newest to oldest"),
				mcp.WithString("project", mcp.Description("Optional project identifier to filter by")),
				mcp.WithString("service", mcp.Description("Optional service identifier to filter by (requires project)")),
				mcp.WithInteger("limit", mcp.Description("Maximum number of logs to return (default: 20, max: 100)")),
				mcp.WithInteger("offset", mcp.Description("Offset for pagination (default: 0)")),
			),
			handler: m.handleGetDevLogs,
		},
		{
			tool: mcp.NewTool("get_dev_logs_by_project",
				mcp.WithDescription("Retrieve development logs for a specific project, ordered from newest to oldest"),
				mcp.WithString("project", mcp.Required(), mcp.Description("Project identifier")),
				mcp.WithInteger("limit", mcp.Description("Maximum number of logs to return (default: 20, max: 100)")),
				mcp.WithInteger("offset", mcp.Description("Offset for pagination (default: 0)")),
			),
			handler: m.handleGetDevLogsByProject,
		},
		{
			tool: mcp.NewTool("get_dev_logs_by_service",
				mcp.WithDescription("Retrieve development logs for a specific project and service, ordered from newest to oldest"),
				mcp.WithString("project", mcp.Required(), mcp.Description("Project identifier")),
				mcp.WithString("service", mcp.Required(), mcp.Description("Service identifier")),
				mcp.WithInteger("limit", mcp.Description("Maximum number of logs to return (default: 20, max: 100)")),
				mcp.WithInteger("offset", mcp.Description("Offset for pagination (default: 0)")),
			),
			handler: m.handleGetDevLogsByService,
		},
		{
			tool: mcp.NewTool("search_dev_logs_fts",
				mcp.WithDescription("Full-text search (FTS5) across dev logs ranked by BM25 relevance score, optionally filtered by project"),
				mcp.WithString("query", mcp.Required(), mcp.Description("Search query terms or phrase")),
				mcp.WithString("project", mcp.Description("Optional project identifier to restrict search")),
				mcp.WithInteger("limit", mcp.Description("Maximum number of results to return (default: 20, max: 100)")),
				mcp.WithInteger("offset", mcp.Description("Offset for pagination (default: 0)")),
			),
			handler: m.handleSearchDevLogsFTS,
		},
		{
			tool: mcp.NewTool("search_dev_logs_fts_by_project",
				mcp.WithDescription("Full-text search (FTS5) across dev logs within a specific project ranked by BM25 relevance score"),
				mcp.WithString("query", mcp.Required(), mcp.Description("Search query terms or phrase")),
				mcp.WithString("project", mcp.Required(), mcp.Description("Project identifier")),
				mcp.WithInteger("limit", mcp.Description("Maximum number of results to return (default: 20, max: 100)")),
				mcp.WithInteger("offset", mcp.Description("Offset for pagination (default: 0)")),
			),
			handler: m.handleSearchDevLogsFTSByProject,
		},

		// Incident Tools
		{
			tool: mcp.NewTool("create_incident",
				mcp.WithDescription("Create a post-mortem incident report recording root cause, resolution, and prevention rules"),
				mcp.WithString("project", mcp.Required(), mcp.Description("Project identifier")),
				mcp.WithString("service", mcp.Description("Service or component within the project")),
				mcp.WithString("title", mcp.Required(), mcp.Description("Incident title")),
				mcp.WithString("root_cause", mcp.Required(), mcp.Description("Detailed root cause analysis")),
				mcp.WithString("resolution", mcp.Required(), mcp.Description("Steps taken to resolve the incident")),
				mcp.WithString("prevention_rules", mcp.Description("Actionable prevention rules, architectural guards, or tests")),
			),
			handler: m.handleCreateIncident,
		},
		{
			tool: mcp.NewTool("get_incidents",
				mcp.WithDescription("Retrieve post-mortem incidents, optionally filtered by project and service"),
				mcp.WithString("project", mcp.Description("Optional project identifier")),
				mcp.WithString("service", mcp.Description("Optional service identifier (requires project)")),
				mcp.WithInteger("limit", mcp.Description("Maximum number of incidents to return (default: 20, max: 100)")),
				mcp.WithInteger("offset", mcp.Description("Offset for pagination (default: 0)")),
			),
			handler: m.handleGetIncidents,
		},

		// Architectural Profile Tools
		{
			tool: mcp.NewTool("create_architectural_profile",
				mcp.WithDescription("Create or update an architectural profile for a project defining architecture style, conventions, core stack, and strict rules"),
				mcp.WithString("project", mcp.Required(), mcp.Description("Project identifier")),
				mcp.WithString("architecture_style", mcp.Required(), mcp.Description("Architecture style (e.g., 'Hexagonal Architecture')")),
				mcp.WithString("conventions", mcp.Description("JSON array or list of architectural conventions and patterns")),
				mcp.WithString("core_stack", mcp.Description("JSON array or list of core technologies, frameworks, and libraries")),
				mcp.WithString("rules", mcp.Description("JSON array or list of strict DO/DON'T constraints")),
			),
			handler: m.handleCreateArchitecturalProfile,
		},
		{
			tool: mcp.NewTool("get_architectural_profile",
				mcp.WithDescription("Retrieve the architectural profile for a project"),
				mcp.WithString("project", mcp.Required(), mcp.Description("Project identifier")),
			),
			handler: m.handleGetArchitecturalProfile,
		},

		// Feature Context Tools
		{
			tool: mcp.NewTool("create_feature_context",
				mcp.WithDescription("Create or update a feature/component backstory with critical invariants and dependencies"),
				mcp.WithString("project", mcp.Required(), mcp.Description("Project identifier")),
				mcp.WithString("feature_name", mcp.Required(), mcp.Description("Feature or component name")),
				mcp.WithString("summary", mcp.Required(), mcp.Description("Summary of the feature's purpose and lifecycle")),
				mcp.WithString("critical_invariants", mcp.Description("JSON array or list of non-negotiable invariants")),
				mcp.WithString("dependencies", mcp.Description("JSON array or list of internal and external dependencies")),
			),
			handler: m.handleCreateFeatureContext,
		},
		{
			tool: mcp.NewTool("get_feature_context",
				mcp.WithDescription("Retrieve feature contexts for a project, optionally filtered by feature name"),
				mcp.WithString("project", mcp.Required(), mcp.Description("Project identifier")),
				mcp.WithString("feature_name", mcp.Description("Optional feature name to fetch a specific feature context")),
			),
			handler: m.handleGetFeatureContext,
		},

		// Code Exemplar Tools
		{
			tool: mcp.NewTool("create_code_exemplar",
				mcp.WithDescription("Create a canonical golden code snippet exemplar for a programming language and topic"),
				mcp.WithString("language", mcp.Required(), mcp.Description("Programming language (e.g., 'go', 'python', 'rust', 'typescript')")),
				mcp.WithString("topic", mcp.Required(), mcp.Description("Pattern or topic name (e.g., 'db_transaction', 'worker_pool')")),
				mcp.WithString("title", mcp.Required(), mcp.Description("Exemplar title")),
				mcp.WithString("code_snippet", mcp.Required(), mcp.Description("Canonical source code snippet")),
				mcp.WithString("description", mcp.Description("Explanation of why this pattern is preferred")),
				mcp.WithString("tags", mcp.Description("Comma-separated tags (e.g., 'concurrency,channels')")),
				mcp.WithInteger("dev_log_id", mcp.Description("Optional ID of the dev log this exemplar was extracted from")),
			),
			handler: m.handleCreateCodeExemplar,
		},
		{
			tool: mcp.NewTool("get_code_exemplars",
				mcp.WithDescription("Retrieve code exemplars filtered by language, topic, or dev log ID"),
				mcp.WithString("language", mcp.Description("Optional language filter")),
				mcp.WithString("topic", mcp.Description("Optional topic filter (requires language)")),
				mcp.WithInteger("dev_log_id", mcp.Description("Optional dev log ID filter")),
				mcp.WithInteger("limit", mcp.Description("Maximum number of exemplars to return (default: 20, max: 100)")),
				mcp.WithInteger("offset", mcp.Description("Offset for pagination (default: 0)")),
			),
			handler: m.handleGetCodeExemplars,
		},
	}

	for _, t := range tools {
		s.AddTool(t.tool, t.handler)
	}

	return nil
}

// --- Tool Handlers ---

func (m *Module) handleSaveDevLog(ctx stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project, err := req.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'project': %v", err)), nil
	}
	title, err := req.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'title': %v", err)), nil
	}
	problem, err := req.RequireString("problem")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'problem': %v", err)), nil
	}
	solution, err := req.RequireString("solution")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'solution': %v", err)), nil
	}

	service := req.GetString("service", "")
	tradeOffs := req.GetString("trade_offs", "")
	tags := parseTags(req, "tags")
	gitRef := req.GetString("git_ref", "")

	log, err := m.repo.CreateDevLog(ctx, repository.CreateDevLogParams{
		Project:   project,
		Service:   service,
		Title:     title,
		Problem:   problem,
		Solution:  solution,
		TradeOffs: tradeOffs,
		Tags:      tags,
		GitRef:    gitRef,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to save dev log: %v", err)), nil
	}

	return marshalResult(log)
}

func (m *Module) handleGetDevLogs(ctx stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := req.GetString("project", "")
	service := req.GetString("service", "")
	limit, offset := extractPagination(req)

	if project != "" && service != "" {
		logs, err := m.repo.ListDevLogsByService(ctx, repository.ListDevLogsByServiceParams{
			Project: project,
			Service: service,
			Limit:   limit,
			Offset:  offset,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list dev logs by service: %v", err)), nil
		}
		if logs == nil {
			logs = []repository.DevLog{}
		}
		return marshalResult(logs)
	}

	if project != "" {
		logs, err := m.repo.ListDevLogsByProject(ctx, repository.ListDevLogsByProjectParams{
			Project: project,
			Limit:   limit,
			Offset:  offset,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list dev logs by project: %v", err)), nil
		}
		if logs == nil {
			logs = []repository.DevLog{}
		}
		return marshalResult(logs)
	}

	logs, err := m.repo.ListDevLogs(ctx, repository.ListDevLogsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list dev logs: %v", err)), nil
	}
	if logs == nil {
		logs = []repository.DevLog{}
	}
	return marshalResult(logs)
}

func (m *Module) handleGetDevLogsByProject(ctx stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project, err := req.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'project': %v", err)), nil
	}
	limit, offset := extractPagination(req)

	logs, err := m.repo.ListDevLogsByProject(ctx, repository.ListDevLogsByProjectParams{
		Project: project,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list dev logs by project: %v", err)), nil
	}
	if logs == nil {
		logs = []repository.DevLog{}
	}
	return marshalResult(logs)
}

func (m *Module) handleGetDevLogsByService(ctx stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project, err := req.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'project': %v", err)), nil
	}
	service, err := req.RequireString("service")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'service': %v", err)), nil
	}
	limit, offset := extractPagination(req)

	logs, err := m.repo.ListDevLogsByService(ctx, repository.ListDevLogsByServiceParams{
		Project: project,
		Service: service,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list dev logs by service: %v", err)), nil
	}
	if logs == nil {
		logs = []repository.DevLog{}
	}
	return marshalResult(logs)
}

func (m *Module) handleSearchDevLogsFTS(ctx stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'query': %v", err)), nil
	}
	project := req.GetString("project", "")
	limit, offset := extractPagination(req)

	if project != "" {
		rows, err := m.repo.SearchDevLogsFTSByProject(ctx, repository.SearchDevLogsFTSByProjectParams{
			Query:   query,
			Project: project,
			Limit:   limit,
			Offset:  offset,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to search dev logs by project: %v", err)), nil
		}
		if rows == nil {
			rows = []repository.SearchDevLogsFTSByProjectRow{}
		}
		return marshalResult(rows)
	}

	rows, err := m.repo.SearchDevLogsFTS(ctx, repository.SearchDevLogsFTSParams{
		Query:  query,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to search dev logs: %v", err)), nil
	}
	if rows == nil {
		rows = []repository.SearchDevLogsFTSRow{}
	}
	return marshalResult(rows)
}

func (m *Module) handleSearchDevLogsFTSByProject(ctx stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'query': %v", err)), nil
	}
	project, err := req.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'project': %v", err)), nil
	}
	limit, offset := extractPagination(req)

	rows, err := m.repo.SearchDevLogsFTSByProject(ctx, repository.SearchDevLogsFTSByProjectParams{
		Query:   query,
		Project: project,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to search dev logs by project: %v", err)), nil
	}
	if rows == nil {
		rows = []repository.SearchDevLogsFTSByProjectRow{}
	}
	return marshalResult(rows)
}

func (m *Module) handleCreateIncident(ctx stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project, err := req.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'project': %v", err)), nil
	}
	title, err := req.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'title': %v", err)), nil
	}
	rootCause, err := req.RequireString("root_cause")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'root_cause': %v", err)), nil
	}
	resolution, err := req.RequireString("resolution")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'resolution': %v", err)), nil
	}

	service := req.GetString("service", "")
	preventionRules := req.GetString("prevention_rules", "")

	incident, err := m.repo.CreateIncident(ctx, repository.CreateIncidentParams{
		Project:         project,
		Service:         service,
		Title:           title,
		RootCause:       rootCause,
		Resolution:      resolution,
		PreventionRules: preventionRules,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create incident: %v", err)), nil
	}

	return marshalResult(incident)
}

func (m *Module) handleGetIncidents(ctx stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := req.GetString("project", "")
	service := req.GetString("service", "")
	limit, offset := extractPagination(req)

	if project != "" && service != "" {
		incidents, err := m.repo.ListIncidentsByService(ctx, repository.ListIncidentsByServiceParams{
			Project: project,
			Service: service,
			Limit:   limit,
			Offset:  offset,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list incidents by service: %v", err)), nil
		}
		if incidents == nil {
			incidents = []repository.Incident{}
		}
		return marshalResult(incidents)
	}

	if project != "" {
		incidents, err := m.repo.ListIncidentsByProject(ctx, repository.ListIncidentsByProjectParams{
			Project: project,
			Limit:   limit,
			Offset:  offset,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list incidents by project: %v", err)), nil
		}
		if incidents == nil {
			incidents = []repository.Incident{}
		}
		return marshalResult(incidents)
	}

	incidents, err := m.repo.ListIncidents(ctx, repository.ListIncidentsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list incidents: %v", err)), nil
	}
	if incidents == nil {
		incidents = []repository.Incident{}
	}
	return marshalResult(incidents)
}

func (m *Module) handleCreateArchitecturalProfile(ctx stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project, err := req.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'project': %v", err)), nil
	}
	architectureStyle, err := req.RequireString("architecture_style")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'architecture_style': %v", err)), nil
	}

	conventions := parseJSONOrString(req, "conventions", "[]")
	coreStack := parseJSONOrString(req, "core_stack", "[]")
	rules := parseJSONOrString(req, "rules", "[]")

	profile, err := m.repo.UpsertArchitecturalProfile(ctx, repository.UpsertArchitecturalProfileParams{
		Project:           project,
		ArchitectureStyle: architectureStyle,
		Conventions:       conventions,
		CoreStack:         coreStack,
		Rules:             rules,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create/update architectural profile: %v", err)), nil
	}

	return marshalResult(profile)
}

func (m *Module) handleGetArchitecturalProfile(ctx stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project, err := req.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'project': %v", err)), nil
	}

	profile, err := m.repo.GetArchitecturalProfileByProject(ctx, project)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return mcp.NewToolResultError(fmt.Sprintf("architectural profile not found for project %q", project)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to get architectural profile: %v", err)), nil
	}

	return marshalResult(profile)
}

func (m *Module) handleCreateFeatureContext(ctx stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project, err := req.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'project': %v", err)), nil
	}
	featureName, err := req.RequireString("feature_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'feature_name': %v", err)), nil
	}
	summary, err := req.RequireString("summary")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'summary': %v", err)), nil
	}

	criticalInvariants := parseJSONOrString(req, "critical_invariants", "[]")
	dependencies := parseJSONOrString(req, "dependencies", "[]")

	fc, err := m.repo.UpsertFeatureContext(ctx, repository.UpsertFeatureContextParams{
		Project:            project,
		FeatureName:        featureName,
		Summary:            summary,
		CriticalInvariants: criticalInvariants,
		Dependencies:       dependencies,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create/update feature context: %v", err)), nil
	}

	return marshalResult(fc)
}

func (m *Module) handleGetFeatureContext(ctx stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project, err := req.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'project': %v", err)), nil
	}
	featureName := req.GetString("feature_name", "")

	if featureName != "" {
		fc, err := m.repo.GetFeatureContextByProjectAndName(ctx, repository.GetFeatureContextByProjectAndNameParams{
			Project:     project,
			FeatureName: featureName,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return mcp.NewToolResultError(fmt.Sprintf("feature context not found for project %q and feature %q", project, featureName)), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("failed to get feature context: %v", err)), nil
		}
		return marshalResult(fc)
	}

	list, err := m.repo.ListFeatureContextsByProject(ctx, project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list feature contexts: %v", err)), nil
	}
	if list == nil {
		list = []repository.FeatureContext{}
	}
	return marshalResult(list)
}

func (m *Module) handleCreateCodeExemplar(ctx stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	language, err := req.RequireString("language")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'language': %v", err)), nil
	}
	topic, err := req.RequireString("topic")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'topic': %v", err)), nil
	}
	title, err := req.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'title': %v", err)), nil
	}
	codeSnippet, err := req.RequireString("code_snippet")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'code_snippet': %v", err)), nil
	}

	description := req.GetString("description", "")
	tags := parseTags(req, "tags")
	devLogID := req.GetInt("dev_log_id", 0)

	var devLogNullID sql.NullInt64
	if devLogID > 0 {
		devLogNullID = sql.NullInt64{Int64: int64(devLogID), Valid: true}
	}

	exemplar, err := m.repo.CreateCodeExemplar(ctx, repository.CreateCodeExemplarParams{
		DevLogID:    devLogNullID,
		Language:    language,
		Topic:       topic,
		Title:       title,
		Description: description,
		CodeSnippet: codeSnippet,
		Tags:        tags,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create code exemplar: %v", err)), nil
	}

	return marshalResult(exemplar)
}

func (m *Module) handleGetCodeExemplars(ctx stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	devLogID := req.GetInt("dev_log_id", 0)
	if devLogID > 0 {
		exemplars, err := m.repo.ListCodeExemplarsByDevLogID(ctx, sql.NullInt64{Int64: int64(devLogID), Valid: true})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list code exemplars by dev log ID: %v", err)), nil
		}
		if exemplars == nil {
			exemplars = []repository.CodeExemplar{}
		}
		return marshalResult(exemplars)
	}

	language := req.GetString("language", "")
	topic := req.GetString("topic", "")
	limit, offset := extractPagination(req)

	if language != "" && topic != "" {
		exemplars, err := m.repo.ListCodeExemplarsByTopic(ctx, repository.ListCodeExemplarsByTopicParams{
			Language: language,
			Topic:    topic,
			Limit:    limit,
			Offset:   offset,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list code exemplars by topic: %v", err)), nil
		}
		if exemplars == nil {
			exemplars = []repository.CodeExemplar{}
		}
		return marshalResult(exemplars)
	}

	if language != "" {
		exemplars, err := m.repo.ListCodeExemplarsByLanguage(ctx, repository.ListCodeExemplarsByLanguageParams{
			Language: language,
			Limit:    limit,
			Offset:   offset,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list code exemplars by language: %v", err)), nil
		}
		if exemplars == nil {
			exemplars = []repository.CodeExemplar{}
		}
		return marshalResult(exemplars)
	}

	exemplars, err := m.repo.ListCodeExemplars(ctx, repository.ListCodeExemplarsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list code exemplars: %v", err)), nil
	}
	if exemplars == nil {
		exemplars = []repository.CodeExemplar{}
	}
	return marshalResult(exemplars)
}

// --- Helper Functions ---

func extractPagination(req mcp.CallToolRequest) (limit int64, offset int64) {
	l := req.GetInt("limit", 20)
	if l <= 0 {
		l = 20
	} else if l > 100 {
		l = 100
	}
	o := req.GetInt("offset", 0)
	if o < 0 {
		o = 0
	}
	return int64(l), int64(o)
}

func parseJSONOrString(r mcp.CallToolRequest, key string, fallback string) string {
	args := r.GetArguments()
	val, ok := args[key]
	if !ok || val == nil {
		return fallback
	}
	switch v := val.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return fallback
		}
		return trimmed
	default:
		b, err := json.Marshal(v)
		if err == nil {
			return string(b)
		}
		return fmt.Sprintf("%v", v)
	}
}

func parseTags(r mcp.CallToolRequest, key string) string {
	args := r.GetArguments()
	val, ok := args[key]
	if !ok || val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		strs := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				strs = append(strs, strings.TrimSpace(s))
			}
		}
		return strings.Join(strs, ",")
	case []string:
		return strings.Join(v, ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func marshalResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
