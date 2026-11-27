-- PostgreSQL 13+. Apply in each service's own database, under migration control.
CREATE TABLE IF NOT EXISTS common_outbox (
 id text PRIMARY KEY, subject text NOT NULL, payload jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), available_at timestamptz NOT NULL DEFAULT now(),
 attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0), published_at timestamptz, dead_at timestamptz
);
CREATE INDEX IF NOT EXISTS common_outbox_pending ON common_outbox (available_at, created_at, id) WHERE published_at IS NULL AND dead_at IS NULL;
CREATE TABLE IF NOT EXISTS common_idempotency (
 tenant_id text NOT NULL, branch_id text NOT NULL, actor_id text NOT NULL, operation text NOT NULL, request_key text NOT NULL,
 fingerprint bytea NOT NULL, response bytea, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (tenant_id,branch_id,actor_id,operation,request_key)
);
CREATE INDEX IF NOT EXISTS common_idempotency_created ON common_idempotency (created_at);
CREATE TABLE IF NOT EXISTS common_audit (
 id text PRIMARY KEY, occurred_at timestamptz NOT NULL DEFAULT now(), tenant_id text NOT NULL, branch_id text NOT NULL,
 actor_id text NOT NULL, request_id text NOT NULL, action text NOT NULL, resource_type text NOT NULL, resource_id text NOT NULL
);
CREATE INDEX IF NOT EXISTS common_audit_resource ON common_audit (tenant_id,branch_id,resource_type,resource_id,occurred_at);
