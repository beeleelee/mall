CREATE TABLE IF NOT EXISTS discount_codes (
    id BIGINT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    value BIGINT NOT NULL,
    min_purchase BIGINT NOT NULL DEFAULT 0,
    max_usages INT NOT NULL DEFAULT 0,
    used_count INT NOT NULL DEFAULT 0,
    expiry TIMESTAMPTZ NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    stackable BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_discount_codes_code ON discount_codes (code);
CREATE INDEX IF NOT EXISTS idx_discount_codes_active ON discount_codes (active);
