-- +goose Up
CREATE TABLE admin_pairing_tokens (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    admin_id uuid NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX admin_pairing_tokens_expiry_idx ON admin_pairing_tokens (expires_at)
    WHERE consumed_at IS NULL;

-- +goose Down
DROP TABLE admin_pairing_tokens;
