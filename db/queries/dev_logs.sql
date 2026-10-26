-- name: CreateDevLog :one
INSERT INTO dev_logs (
    project,
    service,
    title,
    problem,
    solution,
    trade_offs,
    tags,
    git_ref
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
)
RETURNING *;

-- name: GetDevLogByID :one
SELECT * FROM dev_logs
WHERE id = ? LIMIT 1;

-- name: ListDevLogs :many
SELECT * FROM dev_logs
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListDevLogsByProject :many
SELECT * FROM dev_logs
WHERE project = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListDevLogsByService :many
SELECT * FROM dev_logs
WHERE project = ? AND service = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: UpdateDevLog :one
UPDATE dev_logs
SET
    service = ?,
    title = ?,
    problem = ?,
    solution = ?,
    trade_offs = ?,
    tags = ?,
    git_ref = ?
WHERE id = ?
RETURNING *;

-- name: DeleteDevLog :exec
DELETE FROM dev_logs
WHERE id = ?;

-- name: CountDevLogs :one
SELECT COUNT(*) FROM dev_logs;

-- name: CountDevLogsByProject :one
SELECT COUNT(*) FROM dev_logs
WHERE project = ?;

-- name: SearchDevLogsFTS :many
SELECT 
    l.id,
    l.project,
    l.service,
    l.title,
    l.problem,
    l.solution,
    l.trade_offs,
    l.tags,
    l.git_ref,
    l.created_at,
    dev_logs_fts.rank
FROM dev_logs_fts
JOIN dev_logs l ON l.id = dev_logs_fts.rowid
WHERE dev_logs_fts MATCH ?
ORDER BY dev_logs_fts.rank
LIMIT ? OFFSET ?;

-- name: SearchDevLogsFTSByProject :many
SELECT 
    l.id,
    l.project,
    l.service,
    l.title,
    l.problem,
    l.solution,
    l.trade_offs,
    l.tags,
    l.git_ref,
    l.created_at,
    dev_logs_fts.rank
FROM dev_logs_fts
JOIN dev_logs l ON l.id = dev_logs_fts.rowid
WHERE dev_logs_fts MATCH ? AND l.project = ?
ORDER BY dev_logs_fts.rank
LIMIT ? OFFSET ?;

