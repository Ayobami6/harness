-- name: CreateIncident :one
INSERT INTO incidents (
    project,
    service,
    title,
    root_cause,
    resolution,
    prevention_rules
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
)
RETURNING *;

-- name: GetIncidentByID :one
SELECT * FROM incidents
WHERE id = ? LIMIT 1;

-- name: ListIncidents :many
SELECT * FROM incidents
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListIncidentsByProject :many
SELECT * FROM incidents
WHERE project = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListIncidentsByService :many
SELECT * FROM incidents
WHERE project = ? AND service = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: UpdateIncident :one
UPDATE incidents
SET
    service = ?,
    title = ?,
    root_cause = ?,
    resolution = ?,
    prevention_rules = ?
WHERE id = ?
RETURNING *;

-- name: DeleteIncident :exec
DELETE FROM incidents
WHERE id = ?;

-- name: CountIncidentsByProject :one
SELECT COUNT(*) FROM incidents
WHERE project = ?;

