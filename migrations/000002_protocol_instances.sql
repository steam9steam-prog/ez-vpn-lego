-- +goose Up
ALTER TABLE nodes
    ADD COLUMN public_address text NOT NULL DEFAULT '';

CREATE TABLE protocol_instances (
    id uuid PRIMARY KEY,
    node_id uuid NOT NULL REFERENCES nodes(id),
    driver text NOT NULL CHECK (char_length(driver) BETWEEN 1 AND 80),
    schema_version integer NOT NULL CHECK (schema_version > 0),
    status text NOT NULL CHECK (status IN ('active', 'disabled')),
    settings bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (node_id, driver)
);

-- +goose Down
DROP TABLE protocol_instances;
ALTER TABLE nodes DROP COLUMN public_address;

