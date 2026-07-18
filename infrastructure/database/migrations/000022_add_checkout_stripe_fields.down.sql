ALTER TABLE checkout_sessions DROP COLUMN IF EXISTS stripe_session_id;
ALTER TABLE checkout_sessions DROP COLUMN IF EXISTS stripe_payment_intent_id;
ALTER TABLE checkout_sessions DROP COLUMN IF EXISTS stripe_payment_status;
