CREATE TABLE IF NOT EXISTS carts (
    id         BIGINT PRIMARY KEY,
    user_id    BIGINT NOT NULL UNIQUE,
    items      JSONB NOT NULL DEFAULT '[]',
    status     TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_carts_user_id ON carts (user_id);
