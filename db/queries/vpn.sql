-- name: LockVPNBootstrap :exec
SELECT pg_advisory_xact_lock(hashtext('ez-vpn-lego.bootstrap-vpn'));

-- name: CountNodes :one
SELECT count(*) FROM nodes;

-- name: CreateNode :one
INSERT INTO nodes (id, name, status, public_address)
VALUES ($1, $2, 'disabled', $3)
RETURNING id, name, status, created_at, updated_at, public_address;

-- name: CreateProtocolInstance :exec
INSERT INTO protocol_instances (id, node_id, driver, schema_version, status, settings)
VALUES ($1, $2, $3, $4, 'disabled', $5);

-- name: CreateDevice :one
INSERT INTO devices (id, user_id, name, status)
VALUES ($1, $2, $3, 'disabled')
RETURNING id, user_id, name, status, expires_at, created_at, updated_at, deleted_at;

-- name: CreateCredential :exec
INSERT INTO credentials (id, device_id, node_id, driver, schema_version, status, secret)
VALUES ($1, $2, $3, $4, $5, 'disabled', $6);

-- name: CreateConfigRevision :exec
INSERT INTO config_revisions (id, node_id, state, content_hash, content)
VALUES ($1, $2, 'candidate', $3, $4);

-- name: ActivateNode :exec
UPDATE nodes SET status = 'active', updated_at = now() WHERE id = $1;

-- name: ActivateProtocolInstances :exec
UPDATE protocol_instances SET status = 'active', updated_at = now() WHERE node_id = $1;

-- name: ActivateDevice :exec
UPDATE devices SET status = 'active', updated_at = now() WHERE id = $1;

-- name: ActivateCredential :exec
UPDATE credentials SET status = 'active', updated_at = now() WHERE id = $1;

-- name: VerifyConfigRevision :exec
UPDATE config_revisions
SET state = 'verified', verified_at = now()
WHERE id = $1 AND state = 'candidate';

-- name: FailConfigRevision :exec
UPDATE config_revisions SET state = 'failed' WHERE id = $1 AND state = 'candidate';

-- name: FailOperation :exec
UPDATE operations
SET status = 'failed', error_code = $2, finished_at = now(), updated_at = now()
WHERE id = $1 AND status IN ('queued', 'running', 'rolling_back');

-- name: GetActiveRealityInstance :one
SELECT pi.id, pi.node_id, pi.settings, n.public_address
FROM protocol_instances pi
JOIN nodes n ON n.id = pi.node_id
WHERE pi.driver = 'xray-reality-vision'
  AND pi.status = 'active'
  AND n.status = 'active';

-- name: ListActiveRealityCredentials :many
SELECT c.id, c.secret, d.name
FROM credentials c
JOIN devices d ON d.id = c.device_id
JOIN users u ON u.id = d.user_id
WHERE c.node_id = $1
  AND c.driver = 'xray-reality-vision'
  AND c.status = 'active'
  AND d.status = 'active'
  AND u.status = 'active'
  AND d.deleted_at IS NULL
  AND u.deleted_at IS NULL
ORDER BY c.id;

-- name: LockXrayReconcile :exec
SELECT pg_advisory_xact_lock(hashtext('ez-vpn-lego.xray-reconcile'));

-- name: SupersedeVerifiedRevisions :exec
UPDATE config_revisions
SET state = 'superseded'
WHERE node_id = $1 AND state = 'verified';
