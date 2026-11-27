-- Apply after 001. Run through service migration tooling.
CREATE TABLE IF NOT EXISTS common_inbox (
 consumer text NOT NULL, source text NOT NULL, event_id text NOT NULL,
 fingerprint bytea NOT NULL, processed_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (consumer,source,event_id)
);
CREATE INDEX IF NOT EXISTS common_inbox_processed ON common_inbox(processed_at);
ALTER TABLE common_outbox ADD COLUMN IF NOT EXISTS claim_token text;
ALTER TABLE common_outbox ADD COLUMN IF NOT EXISTS claim_until timestamptz;
ALTER TABLE common_audit ADD COLUMN IF NOT EXISTS change_data text;
ALTER TABLE common_audit ADD COLUMN IF NOT EXISTS change_mac bytea;
ALTER TABLE common_audit ADD COLUMN IF NOT EXISTS mac_key_id text;
