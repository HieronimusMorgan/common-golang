-- Example only. Adapt table names, service roles and policies to each consumer.
-- Execute as a migration owner, not the runtime application role.
-- CREATE ROLE asset_runtime NOLOGIN NOSUPERUSER NOBYPASSRLS;
CREATE TABLE asset_example (
    tenant_id text NOT NULL,
    branch_id text NOT NULL,
    id text NOT NULL,
    PRIMARY KEY (tenant_id, branch_id, id)
);
ALTER TABLE asset_example ENABLE ROW LEVEL SECURITY;
ALTER TABLE asset_example FORCE ROW LEVEL SECURITY;
CREATE POLICY asset_scope ON asset_example
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')
       AND branch_id = NULLIF(current_setting('app.branch_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')
       AND branch_id = NULLIF(current_setting('app.branch_id', true), ''));
-- GRANT SELECT, INSERT, UPDATE, DELETE ON asset_example TO asset_runtime;
-- REVOKE UPDATE, DELETE, TRUNCATE ON common_audit FROM asset_runtime;
-- GRANT INSERT ON common_audit TO asset_runtime;
-- Ensure runtime roles cannot SET ROLE to owner and have no inherited bypass privileges.
-- RLS does not protect against arbitrary SQL execution under a trusted role that can set app.*.
