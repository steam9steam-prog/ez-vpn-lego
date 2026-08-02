-- name: CreateOperation :one
INSERT INTO operations (
    id,
    kind,
    status,
    requested_by,
    idempotency_key,
    input
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (requested_by, idempotency_key) DO NOTHING
RETURNING id, kind, status, requested_by, idempotency_key, input, result,
    error_code, attempts, available_at, started_at, finished_at, created_at,
    updated_at;

-- name: GetOperationByIdempotencyKey :one
SELECT id, kind, status, requested_by, idempotency_key, input, result,
    error_code, attempts, available_at, started_at, finished_at, created_at,
    updated_at
FROM operations
WHERE requested_by = $1 AND idempotency_key = $2;

-- name: CompleteOperation :one
UPDATE operations
SET status = 'succeeded', result = $2, finished_at = now(), updated_at = now()
WHERE id = $1 AND status IN ('queued', 'running')
RETURNING id, kind, status, requested_by, idempotency_key, input, result,
    error_code, attempts, available_at, started_at, finished_at, created_at,
    updated_at;

