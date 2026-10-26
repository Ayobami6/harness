-- name: CreateFeatureContext :one
INSERT INTO feature_contexts (
    project,
    feature_name,
    summary,
    critical_invariants,
    dependencies
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?
)
RETURNING *;

-- name: UpsertFeatureContext :one
INSERT INTO feature_contexts (
    project,
    feature_name,
    summary,
    critical_invariants,
    dependencies,
    updated_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    CURRENT_TIMESTAMP
)
ON CONFLICT(project, feature_name) DO UPDATE SET
    summary = excluded.summary,
    critical_invariants = excluded.critical_invariants,
    dependencies = excluded.dependencies,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetFeatureContextByID :one
SELECT * FROM feature_contexts
WHERE id = ? LIMIT 1;

-- name: GetFeatureContextByProjectAndName :one
SELECT * FROM feature_contexts
WHERE project = ? AND feature_name = ? LIMIT 1;

-- name: ListFeatureContextsByProject :many
SELECT * FROM feature_contexts
WHERE project = ?
ORDER BY feature_name ASC;

-- name: UpdateFeatureContext :one
UPDATE feature_contexts
SET
    summary = ?,
    critical_invariants = ?,
    dependencies = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteFeatureContext :exec
DELETE FROM feature_contexts
WHERE id = ?;

-- name: DeleteFeatureContextByProjectAndName :exec
DELETE FROM feature_contexts
WHERE project = ? AND feature_name = ?;

