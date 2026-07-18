ALTER TABLE checkout_sessions ADD COLUMN IF NOT EXISTS stripe_session_id TEXT NOT NULL DEFAULT '';
ALTER TABLE checkout_sessions ADD COLUMN IF NOT EXISTS stripe_payment_intent_id TEXT NOT NULL DEFAULT '';
ALTER TABLE checkout_sessions ADD COLUMN IF NOT EXISTS stripe_payment_status TEXT NOT NULL DEFAULT '';
