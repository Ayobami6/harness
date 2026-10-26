-- name: CreateCodeExemplar :one
INSERT INTO code_exemplars (
    dev_log_id,
    language,
    topic,
    title,
    description,
    code_snippet,
    tags
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
)
RETURNING *;

-- name: GetCodeExemplarByID :one
SELECT * FROM code_exemplars
WHERE id = ? LIMIT 1;

-- name: ListCodeExemplars :many
SELECT * FROM code_exemplars
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;

-- name: ListCodeExemplarsByLanguage :many
SELECT * FROM code_exemplars
WHERE language = ?
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;

-- name: ListCodeExemplarsByTopic :many
SELECT * FROM code_exemplars
WHERE language = ? AND topic = ?
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;

-- name: ListCodeExemplarsByDevLogID :many
SELECT * FROM code_exemplars
WHERE dev_log_id = ?
ORDER BY created_at ASC;

-- name: UpdateCodeExemplar :one
UPDATE code_exemplars
SET
    dev_log_id = ?,
    language = ?,
    topic = ?,
    title = ?,
    description = ?,
    code_snippet = ?,
    tags = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteCodeExemplar :exec
DELETE FROM code_exemplars
WHERE id = ?;

