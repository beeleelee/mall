CREATE TABLE IF NOT EXISTS products (
    id             BIGINT PRIMARY KEY,
    sku            VARCHAR(255) NOT NULL UNIQUE,
    name           VARCHAR(500) NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    category       VARCHAR(255) NOT NULL DEFAULT '',
    price_amount   BIGINT NOT NULL,
    price_currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    status         VARCHAR(50) NOT NULL DEFAULT 'active',
    attributes     JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_products_sku ON products (sku);
CREATE INDEX idx_products_status ON products (status);
CREATE INDEX idx_products_category ON products (category);
CREATE INDEX idx_products_created_at ON products (created_at DESC);
