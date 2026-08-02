-- name: CreateUser :one
INSERT INTO users (id, name, status)
VALUES ($1, $2, 'active')
RETURNING id, name, status, created_at, updated_at, deleted_at;

-- name: GetUser :one
SELECT id, name, status, created_at, updated_at, deleted_at
FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListUsers :many
SELECT id, name, status, created_at, updated_at, deleted_at
FROM users
WHERE deleted_at IS NULL
ORDER BY created_at, id;

