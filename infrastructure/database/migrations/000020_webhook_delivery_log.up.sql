CREATE TABLE IF NOT EXISTS webhook_delivery_log (
    id         BIGINT PRIMARY KEY,
    webhook_id BIGINT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event      VARCHAR(255) NOT NULL,
    payload    JSONB NOT NULL DEFAULT '{}',
    status     VARCHAR(50) NOT NULL DEFAULT 'failed',
    error      TEXT,
    attempts   INT NOT NULL DEFAULT 0,
    next_retry TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_delivery_log_status ON webhook_delivery_log (status);
CREATE INDEX IF NOT EXISTS idx_webhook_delivery_log_next_retry ON webhook_delivery_log (next_retry);
CREATE INDEX IF NOT EXISTS idx_webhook_delivery_log_webhook ON webhook_delivery_log (webhook_id);
