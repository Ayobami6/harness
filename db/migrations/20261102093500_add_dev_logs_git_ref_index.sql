-- +goose Up
CREATE INDEX IF NOT EXISTS idx_dev_logs_project_git_ref
ON dev_logs(project, git_ref);

-- +goose Down
DROP INDEX IF EXISTS idx_dev_logs_project_git_ref;

