-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

CREATE TABLE IF NOT EXISTS decision_audit (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL,
    decision_id VARCHAR(128) NOT NULL UNIQUE,
    asset_id VARCHAR(64) NOT NULL,
    model VARCHAR(64) NOT NULL,
    state_payload JSONB NOT NULL,
    raw_response JSONB NOT NULL,
    governed_decision JSONB NOT NULL,
    policy_version VARCHAR(32) NOT NULL,
    requires_approval BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_decision_audit_tenant_asset 
    ON decision_audit (tenant_id, asset_id, created_at DESC);

ALTER TABLE decision_audit ENABLE ROW LEVEL SECURITY;

CREATE POLICY decision_audit_tenant_isolation ON decision_audit
    USING (tenant_id = NULLIF(current_setting('app.current_tenant', true), ''));

-- +goose Down
-- SQL in section 'Down' is executed when this migration is reverted

DROP POLICY IF EXISTS decision_audit_tenant_isolation ON decision_audit;
DROP TABLE IF EXISTS decision_audit;
