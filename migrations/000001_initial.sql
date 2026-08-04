-- +goose Up
CREATE TABLE admins (
    id uuid PRIMARY KEY,
    role text NOT NULL CHECK (role IN ('owner', 'operator', 'viewer')),
    status text NOT NULL CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX one_active_owner
    ON admins ((role))
    WHERE role = 'owner' AND status = 'active';

CREATE TABLE admin_identities (
    id uuid PRIMARY KEY,
    admin_id uuid NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    provider text NOT NULL,
    subject text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, subject),
    UNIQUE (admin_id, provider)
);

CREATE TABLE users (
    id uuid PRIMARY KEY,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    status text NOT NULL CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE TABLE devices (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    status text NOT NULL CHECK (status IN ('active', 'disabled')),
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX devices_user_id_idx ON devices (user_id);

CREATE TABLE nodes (
    id uuid PRIMARY KEY,
    name text NOT NULL UNIQUE CHECK (char_length(name) BETWEEN 1 AND 80),
    status text NOT NULL CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE credentials (
    id uuid PRIMARY KEY,
    device_id uuid NOT NULL REFERENCES devices(id),
    node_id uuid NOT NULL REFERENCES nodes(id),
    driver text NOT NULL CHECK (char_length(driver) BETWEEN 1 AND 80),
    schema_version integer NOT NULL CHECK (schema_version > 0),
    status text NOT NULL CHECK (status IN ('active', 'disabled')),
    secret bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (device_id, node_id, driver)
);

CREATE INDEX credentials_node_id_idx ON credentials (node_id);

CREATE TABLE config_revisions (
    id uuid PRIMARY KEY,
    node_id uuid NOT NULL REFERENCES nodes(id),
    sequence bigint GENERATED ALWAYS AS IDENTITY,
    state text NOT NULL CHECK (state IN ('candidate', 'verified', 'failed', 'superseded')),
    content_hash text NOT NULL CHECK (char_length(content_hash) = 64),
    content bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    verified_at timestamptz,
    UNIQUE (node_id, sequence)
);

CREATE UNIQUE INDEX one_verified_revision_per_node
    ON config_revisions (node_id)
    WHERE state = 'verified';

CREATE TABLE operations (
    id uuid PRIMARY KEY,
    kind text NOT NULL CHECK (char_length(kind) BETWEEN 1 AND 120),
    status text NOT NULL CHECK (
        status IN ('queued', 'running', 'succeeded', 'failed', 'rolling_back', 'rolled_back')
    ),
    requested_by uuid REFERENCES admins(id),
    idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 16 AND 128),
    input jsonb NOT NULL DEFAULT '{}'::jsonb,
    result jsonb,
    error_code text,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (requested_by, idempotency_key)
);

CREATE INDEX operations_claim_idx ON operations (status, available_at, created_at);

CREATE TABLE audit_events (
    sequence bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id uuid NOT NULL UNIQUE,
    actor_admin_id uuid REFERENCES admins(id),
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id uuid,
    outcome text NOT NULL CHECK (outcome IN ('accepted', 'succeeded', 'denied', 'failed')),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE outbox_events (
    id uuid PRIMARY KEY,
    topic text NOT NULL,
    aggregate_type text NOT NULL,
    aggregate_id uuid NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error text
);

CREATE INDEX outbox_unpublished_idx
    ON outbox_events (created_at)
    WHERE published_at IS NULL;

CREATE TABLE releases (
    version text PRIMARY KEY,
    channel text NOT NULL CHECK (channel IN ('stable', 'beta')),
    manifest jsonb NOT NULL,
    status text NOT NULL CHECK (status IN ('available', 'staged', 'installed', 'failed', 'rolled_back')),
    created_at timestamptz NOT NULL DEFAULT now(),
    installed_at timestamptz
);

CREATE TABLE backups (
    id uuid PRIMARY KEY,
    format_version integer NOT NULL CHECK (format_version > 0),
    path text NOT NULL,
    checksum text NOT NULL CHECK (char_length(checksum) = 64),
    status text NOT NULL CHECK (status IN ('creating', 'verified', 'failed', 'restored')),
    created_at timestamptz NOT NULL DEFAULT now(),
    verified_at timestamptz
);

-- +goose Down
DROP TABLE backups;
DROP TABLE releases;
DROP TABLE outbox_events;
DROP TABLE audit_events;
DROP TABLE operations;
DROP TABLE config_revisions;
DROP TABLE credentials;
DROP TABLE nodes;
DROP TABLE devices;
DROP TABLE users;
DROP TABLE admin_identities;
DROP TABLE admins;
