CREATE TABLE IF NOT EXISTS users (
    id              BIGINT PRIMARY KEY,
    email           VARCHAR(255) NOT NULL UNIQUE,
    name            VARCHAR(500) NOT NULL,
    password_hash   TEXT NOT NULL,
    status          VARCHAR(50) NOT NULL DEFAULT 'active',
    roles           JSONB NOT NULL DEFAULT '["customer"]',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_status ON users (status);
