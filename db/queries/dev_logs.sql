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
    CAST(bm25(dev_logs_fts) AS REAL) AS fts_rank
FROM dev_logs_fts(sqlc.arg(query))
JOIN dev_logs l ON l.id = dev_logs_fts.rowid
ORDER BY fts_rank
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

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
    CAST(bm25(dev_logs_fts) AS REAL) AS fts_rank
FROM dev_logs_fts(sqlc.arg(query))
JOIN dev_logs l ON l.id = dev_logs_fts.rowid
WHERE l.project = sqlc.arg(project)
ORDER BY fts_rank
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: ExistsDevLogByGitRef :one
SELECT CAST(EXISTS(
    SELECT 1 FROM dev_logs
    WHERE project = sqlc.arg(project)
      AND git_ref != ''
      AND (
          git_ref = sqlc.arg(full_sha)
          OR git_ref = sqlc.arg(short_sha)
          OR (LENGTH(git_ref) >= 7 AND sqlc.arg(full_sha) LIKE git_ref || '%')
          OR (LENGTH(sqlc.arg(full_sha)) >= 7 AND git_ref LIKE sqlc.arg(full_sha) || '%')
      )
) AS INTEGER) AS found;

-- name: GetDevLogByGitRef :one
SELECT * FROM dev_logs
WHERE project = sqlc.arg(project)
  AND git_ref != ''
  AND (
      git_ref = sqlc.arg(git_ref)
      OR (LENGTH(git_ref) >= 7 AND sqlc.arg(git_ref) LIKE git_ref || '%')
      OR (LENGTH(sqlc.arg(git_ref)) >= 7 AND git_ref LIKE sqlc.arg(git_ref) || '%')
  )
ORDER BY created_at DESC
LIMIT 1;


