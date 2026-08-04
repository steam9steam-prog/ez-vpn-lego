-- name: CreateTelegramPairingToken :execrows
INSERT INTO admin_pairing_tokens (token_hash, admin_id, expires_at)
SELECT sqlc.arg(token_hash), id, sqlc.arg(expires_at) FROM admins
WHERE id = sqlc.arg(admin_id) AND status = 'active';

-- name: ConsumeTelegramPairingToken :one
UPDATE admin_pairing_tokens
SET consumed_at = now()
WHERE token_hash = $1
  AND consumed_at IS NULL
  AND expires_at > now()
RETURNING admin_id;

-- name: CreateTelegramIdentity :exec
INSERT INTO admin_identities (id, admin_id, provider, subject)
VALUES ($1, $2, 'telegram', $3);

-- name: ResolveTelegramIdentity :one
SELECT a.*
FROM admins a
JOIN admin_identities i ON i.admin_id = a.id
WHERE i.provider = 'telegram' AND i.subject = $1 AND a.status = 'active';
