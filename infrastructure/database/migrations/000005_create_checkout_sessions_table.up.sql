CREATE TABLE IF NOT EXISTS checkout_sessions (
    id               BIGINT PRIMARY KEY,
    user_id          BIGINT NOT NULL,
    cart_id          BIGINT NOT NULL,
    cart_snapshot    JSONB NOT NULL DEFAULT '{}',
    shipping_address JSONB,
    billing_address  JSONB,
    shipping_option  JSONB,
    payment_handler  TEXT NOT NULL DEFAULT '',
    subtotal         BIGINT NOT NULL DEFAULT 0,
    shipping_cost    BIGINT NOT NULL DEFAULT 0,
    tax_amount       BIGINT NOT NULL DEFAULT 0,
    grand_total      BIGINT NOT NULL DEFAULT 0,
    status           TEXT NOT NULL DEFAULT 'incomplete',
    completed_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_checkout_sessions_user_id ON checkout_sessions (user_id);
