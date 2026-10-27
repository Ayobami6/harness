# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Standard library HTTP `/healthz` endpoint verifying service liveness and SQLite connectivity (`internal/health`).
- Graceful HTTP server shutdown handling `SIGINT` and `SIGTERM` signals in `cmd/main.go` and `internal/mcp`.
- Implemented MCP context tools in `internal/tools/context`:
  - `save_dev_log` (and `create_dev_log` alias) to record development stream items with problems, solutions, trade-offs, and git refs.
  - `get_dev_logs`, `get_dev_logs_by_project`, and `get_dev_logs_by_service` to list development logs with pagination.
  - `search_dev_logs_fts` and `search_dev_logs_fts_by_project` for ranked SQLite FTS5 BM25 search.
  - `create_incident` and `get_incidents` for post-mortem recording and retrieval.
  - `create_architectural_profile` and `get_architectural_profile` for project conventions and constraints.
  - `create_feature_context` and `get_feature_context` for component backstories and critical invariants.
  - `create_code_exemplar` and `get_code_exemplars` for language and topic golden snippets.
- Wired MCP server and context tool module registration into `cmd/main.go`.

### Changed
- Configured Gemini PR review workflow to preserve complete review history by posting a new comment per review iteration with the commit SHA instead of editing previous comments.

