-- name: GetActiveAdmin :one
SELECT id, role, status, created_at, updated_at
FROM admins
WHERE id = $1 AND status = 'active';

-- name: LockOwnerBootstrap :exec
SELECT pg_advisory_xact_lock(hashtext('ez-vpn-lego.bootstrap-owner'));

-- name: CountAdmins :one
SELECT count(*) FROM admins;

-- name: CreateOwner :one
INSERT INTO admins (id, role, status)
VALUES ($1, 'owner', 'active')
RETURNING id, role, status, created_at, updated_at;
