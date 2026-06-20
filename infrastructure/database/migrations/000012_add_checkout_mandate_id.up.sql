ALTER TABLE checkout_sessions ADD COLUMN IF NOT EXISTS mandate_id BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_checkout_sessions_mandate_id ON checkout_sessions (mandate_id);
