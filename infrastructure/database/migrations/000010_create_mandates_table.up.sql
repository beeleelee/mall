CREATE TABLE IF NOT EXISTS mandates (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    status TEXT NOT NULL,
    scope JSONB NOT NULL,
    signature TEXT NOT NULL DEFAULT '',
    token TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mandates_user_id ON mandates (user_id);
CREATE INDEX IF NOT EXISTS idx_mandates_status ON mandates (status);
