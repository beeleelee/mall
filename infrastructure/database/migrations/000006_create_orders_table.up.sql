CREATE TABLE IF NOT EXISTS orders (
    id               BIGINT PRIMARY KEY,
    user_id          BIGINT NOT NULL,
    checkout_id      BIGINT NOT NULL,
    cart_id          BIGINT NOT NULL,
    items            JSONB NOT NULL DEFAULT '[]',
    shipping_address JSONB NOT NULL DEFAULT '{}',
    billing_address  JSONB NOT NULL DEFAULT '{}',
    shipping_option  JSONB NOT NULL DEFAULT '{}',
    payment_handler  TEXT NOT NULL DEFAULT '',
    subtotal         BIGINT NOT NULL DEFAULT 0,
    shipping_cost    BIGINT NOT NULL DEFAULT 0,
    tax_amount       BIGINT NOT NULL DEFAULT 0,
    grand_total      BIGINT NOT NULL DEFAULT 0,
    status           TEXT NOT NULL DEFAULT 'confirmed',
    tracking_number  TEXT NOT NULL DEFAULT '',
    carrier          TEXT NOT NULL DEFAULT '',
    confirmed_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    processing_at    TIMESTAMPTZ,
    shipped_at       TIMESTAMPTZ,
    delivered_at     TIMESTAMPTZ,
    returned_at      TIMESTAMPTZ,
    cancelled_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders (user_id);
