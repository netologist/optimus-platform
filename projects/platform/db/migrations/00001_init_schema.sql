-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

CREATE EXTENSION IF NOT EXISTS vector;

-- 1. Tenants Table
CREATE TABLE IF NOT EXISTS tenants (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Assets Table
CREATE TABLE IF NOT EXISTS assets (
    id VARCHAR(64) NOT NULL,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    asset_type VARCHAR(128) NOT NULL,
    plant VARCHAR(128) NOT NULL,
    status VARCHAR(64) NOT NULL DEFAULT 'OPERATIONAL',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id)
);

-- 3. Operational Signals Table
CREATE TABLE IF NOT EXISTS signals (
    id VARCHAR(64) NOT NULL,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    asset_id VARCHAR(64) NOT NULL,
    symptom TEXT NOT NULL,
    status VARCHAR(64) NOT NULL DEFAULT 'INGESTED',
    workflow_id VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id)
);

-- 4. Work Orders Table
CREATE TABLE IF NOT EXISTS work_orders (
    id VARCHAR(64) NOT NULL,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    asset_id VARCHAR(64) NOT NULL,
    priority VARCHAR(32) NOT NULL,
    status VARCHAR(64) NOT NULL DEFAULT 'OPEN',
    idempotency_key VARCHAR(128) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id)
);

-- 5. Transactional Outbox Table
CREATE TABLE IF NOT EXISTS outbox (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    event_type VARCHAR(128) NOT NULL,
    correlation_id VARCHAR(128) NOT NULL,
    traceparent VARCHAR(255),
    payload JSONB NOT NULL,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON outbox (created_at) WHERE published_at IS NULL;

-- 6. Technical Documents (PLM)
CREATE TABLE IF NOT EXISTS documents (
    id VARCHAR(64) NOT NULL,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    doc_type VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id)
);

-- 7. Document Chunks & Embeddings (Hybrid Search: pgvector + FTS)
CREATE TABLE IF NOT EXISTS document_chunks (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    document_id VARCHAR(64) NOT NULL,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    tsv TSVECTOR GENERATED ALWAYS AS (to_tsvector('english', content)) STORED,
    embedding vector(384),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_chunks_tsv ON document_chunks USING GIN(tsv);
CREATE INDEX IF NOT EXISTS idx_chunks_embedding ON document_chunks USING hnsw (embedding vector_cosine_ops);

-- ============================================================================
-- Row-Level Security (RLS) Policies
-- ============================================================================
ALTER TABLE assets ENABLE ROW LEVEL SECURITY;
ALTER TABLE signals ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_chunks ENABLE ROW LEVEL SECURITY;

CREATE POLICY assets_tenant_isolation ON assets
    USING (tenant_id = NULLIF(current_setting('app.current_tenant', true), ''));

CREATE POLICY signals_tenant_isolation ON signals
    USING (tenant_id = NULLIF(current_setting('app.current_tenant', true), ''));

CREATE POLICY work_orders_tenant_isolation ON work_orders
    USING (tenant_id = NULLIF(current_setting('app.current_tenant', true), ''));

CREATE POLICY outbox_tenant_isolation ON outbox
    USING (tenant_id = NULLIF(current_setting('app.current_tenant', true), ''));

CREATE POLICY documents_tenant_isolation ON documents
    USING (tenant_id = NULLIF(current_setting('app.current_tenant', true), ''));

CREATE POLICY document_chunks_tenant_isolation ON document_chunks
    USING (tenant_id = NULLIF(current_setting('app.current_tenant', true), ''));

-- +goose Down
-- SQL in section 'Down' is executed when this migration is reverted

DROP POLICY IF EXISTS document_chunks_tenant_isolation ON document_chunks;
DROP POLICY IF EXISTS documents_tenant_isolation ON documents;
DROP POLICY IF EXISTS outbox_tenant_isolation ON outbox;
DROP POLICY IF EXISTS work_orders_tenant_isolation ON work_orders;
DROP POLICY IF EXISTS signals_tenant_isolation ON signals;
DROP POLICY IF EXISTS assets_tenant_isolation ON assets;

DROP TABLE IF EXISTS document_chunks;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS work_orders;
DROP TABLE IF EXISTS signals;
DROP TABLE IF EXISTS assets;
DROP TABLE IF EXISTS tenants;
DROP EXTENSION IF EXISTS vector;
