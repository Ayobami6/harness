-- +goose Up

-- 1. Daily Development Stream
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

-- 2. Full-Text Search (FTS5) Virtual Table & Triggers
CREATE VIRTUAL TABLE IF NOT EXISTS dev_logs_fts USING fts5(
    title,
    problem,
    solution,
    trade_offs,
    tags,
    content='dev_logs',
    content_rowid='id'
);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS dev_logs_ai AFTER INSERT ON dev_logs BEGIN
    INSERT INTO dev_logs_fts(rowid, title, problem, solution, trade_offs, tags)
    VALUES (new.id, new.title, new.problem, new.solution, new.trade_offs, new.tags);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS dev_logs_ad AFTER DELETE ON dev_logs BEGIN
    INSERT INTO dev_logs_fts(dev_logs_fts, rowid, title, problem, solution, trade_offs, tags)
    VALUES('delete', old.id, old.title, old.problem, old.solution, old.trade_offs, old.tags);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS dev_logs_au AFTER UPDATE ON dev_logs BEGIN
    INSERT INTO dev_logs_fts(dev_logs_fts, rowid, title, problem, solution, trade_offs, tags)
    VALUES('delete', old.id, old.title, old.problem, old.solution, old.trade_offs, old.tags);
    INSERT INTO dev_logs_fts(rowid, title, problem, solution, trade_offs, tags)
    VALUES (new.id, new.title, new.problem, new.solution, new.trade_offs, new.tags);
END;
-- +goose StatementEnd

-- 3. Architectural Rules & Preferred Stack
CREATE TABLE IF NOT EXISTS architectural_profiles (
    project TEXT PRIMARY KEY,
    architecture_style TEXT NOT NULL,
    conventions TEXT NOT NULL,       -- JSON array of conventions & rules
    core_stack TEXT NOT NULL,        -- JSON array of libraries/technologies
    rules TEXT NOT NULL DEFAULT '[]',-- JSON array of strict DO/DON'T constraints
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 4. Post-Mortems, Incidents & Failure Modes
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

-- 5. Component Lifecycle & Backstory
CREATE TABLE IF NOT EXISTS feature_contexts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project TEXT NOT NULL,
    feature_name TEXT NOT NULL,
    summary TEXT NOT NULL,
    critical_invariants TEXT NOT NULL,       -- JSON array of non-negotiable invariants
    dependencies TEXT NOT NULL DEFAULT '[]', -- JSON array of internal/external dependencies
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project, feature_name)
);

-- 6. Canonical Code Exemplars (Golden Snippets)
CREATE TABLE IF NOT EXISTS code_exemplars (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    dev_log_id INTEGER REFERENCES dev_logs(id) ON DELETE SET NULL,
    language TEXT NOT NULL,                  -- e.g. "go", "python", "sql"
    topic TEXT NOT NULL,                     -- e.g. "db_transaction", "worker_pool", "error_wrapping"
    title TEXT NOT NULL,                     -- e.g. "Atomic multi-table balance ledger write"
    description TEXT NOT NULL DEFAULT '',    -- Brief explanation of why this pattern is preferred
    code_snippet TEXT NOT NULL,              -- The canonical source code snippet
    tags TEXT NOT NULL DEFAULT '',           -- Comma-separated tags (e.g. "concurrency,channels")
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_code_exemplars_lookup ON code_exemplars(language, topic);
CREATE INDEX IF NOT EXISTS idx_code_exemplars_dev_log ON code_exemplars(dev_log_id);

-- +goose Down
DROP TABLE IF EXISTS code_exemplars;
DROP TABLE IF EXISTS feature_contexts;
DROP TABLE IF EXISTS incidents;
DROP TABLE IF EXISTS architectural_profiles;
DROP TRIGGER IF EXISTS dev_logs_au;
DROP TRIGGER IF EXISTS dev_logs_ad;
DROP TRIGGER IF EXISTS dev_logs_ai;
DROP TABLE IF EXISTS dev_logs_fts;
DROP TABLE IF EXISTS dev_logs;
