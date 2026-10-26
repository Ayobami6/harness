-- name: CreateArchitecturalProfile :one
INSERT INTO architectural_profiles (
    project,
    architecture_style,
    conventions,
    core_stack,
    rules
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?
)
RETURNING *;

-- name: UpsertArchitecturalProfile :one
INSERT INTO architectural_profiles (
    project,
    architecture_style,
    conventions,
    core_stack,
    rules,
    updated_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    CURRENT_TIMESTAMP
)
ON CONFLICT(project) DO UPDATE SET
    architecture_style = excluded.architecture_style,
    conventions = excluded.conventions,
    core_stack = excluded.core_stack,
    rules = excluded.rules,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetArchitecturalProfileByProject :one
SELECT * FROM architectural_profiles
WHERE project = ? LIMIT 1;

-- name: ListArchitecturalProfiles :many
SELECT * FROM architectural_profiles
ORDER BY project ASC;

-- name: UpdateArchitecturalProfile :one
UPDATE architectural_profiles
SET
    architecture_style = ?,
    conventions = ?,
    core_stack = ?,
    rules = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE project = ?
RETURNING *;

-- name: DeleteArchitecturalProfile :exec
DELETE FROM architectural_profiles
WHERE project = ?;

