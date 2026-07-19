-- +goose Up
CREATE TABLE clients (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email           text UNIQUE NOT NULL,
    display_name    text NOT NULL,
    api_key_hash    text NOT NULL DEFAULT '',
    api_key_prefix  text NOT NULL DEFAULT '',
    rate_limit_rpm  int NOT NULL DEFAULT 600,
    created_at      timestamptz NOT NULL DEFAULT now(),
    verified_at     timestamptz,
    revoked_at      timestamptz
);

CREATE INDEX idx_clients_api_key_hash ON clients (api_key_hash);
CREATE INDEX idx_clients_email ON clients (email);

CREATE TABLE verification_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email       text NOT NULL,
    token_hash  text NOT NULL,
    type        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz
);

CREATE INDEX idx_verification_tokens_token_hash ON verification_tokens (token_hash);

-- +goose Down
DROP TABLE IF EXISTS verification_tokens;
DROP TABLE IF EXISTS clients;
