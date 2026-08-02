-- name: CreateAuditEvent :exec
INSERT INTO audit_events (
    id,
    actor_admin_id,
    action,
    resource_type,
    resource_id,
    outcome,
    metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: CreateOutboxEvent :exec
INSERT INTO outbox_events (
    id,
    topic,
    aggregate_type,
    aggregate_id,
    payload
)
VALUES ($1, $2, $3, $4, $5);

