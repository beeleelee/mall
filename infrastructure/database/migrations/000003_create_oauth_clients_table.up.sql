CREATE TABLE IF NOT EXISTS oauth_clients (
    id            BIGINT PRIMARY KEY,
    client_id     TEXT NOT NULL UNIQUE,
    secret_hash   TEXT NOT NULL,
    redirect_uris TEXT NOT NULL DEFAULT '[]',
    scopes        TEXT NOT NULL DEFAULT '[]',
    status        TEXT NOT NULL DEFAULT 'active',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
    code         TEXT PRIMARY KEY,
    client_id    TEXT NOT NULL,
    user_id      BIGINT NOT NULL,
    redirect_uri TEXT NOT NULL,
    scope        TEXT NOT NULL DEFAULT '',
    expires_at   TIMESTAMPTZ NOT NULL,
    used         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
    id         TEXT PRIMARY KEY,
    client_id  TEXT NOT NULL,
    user_id    BIGINT NOT NULL,
    scope      TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    revoked    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_oauth_clients_client_id ON oauth_clients (client_id);
CREATE INDEX IF NOT EXISTS idx_oauth_authorization_codes_client_id ON oauth_authorization_codes (client_id);
CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_client_id ON oauth_refresh_tokens (client_id);
