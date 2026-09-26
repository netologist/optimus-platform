# TD-0002: Object Storage Ingestion - Real S3 / MinIO Mock for Enterprise Knowledge Ingestion

- **Status:** Open / Accepted Tech Debt
- **Impact Area:** `deployments/tenants/acme/knowledge_source.yaml`, `projects/ai-runtime` (`optimus_ai.rag.ingest`), `projects/operator`
- **Target Release:** Phase 9 (Enterprise Hardening & External Storage Integration)
- **Component Owner:** Platform & AI Infrastructure

---

## 1. Problem & Current State

The `EnterpriseKnowledgeSource` CRD (`deployments/tenants/acme/knowledge_source.yaml`) models declarative document synchronization from external object storage:
```yaml
spec:
  tenantRef: acme
  sourceType: s3
  endpoint: "https://s3.eu-west-1.amazonaws.com/acme-plm-docs/pumps"
  credentialsSecretRef:
    name: acme-plm-s3-credentials
```

When reconciled, `EnterpriseKnowledgeSourceController` spawns a Kubernetes `Job` running `python -m optimus_ai.rag.ingest`.

Currently, `ingest.py` defaults to local fixture parsing (`PLM-COOL-4021`) and persists vector embeddings directly to PostgreSQL `pgvector`. It does not execute real S3 API calls (`GetObject`, `ListObjectsV2`) or connect to a local S3-compatible object store (such as MinIO).

---

## 2. Why MinIO Was Deferred for Local/CI Kind

1. **Cluster Overhead & CI Stability:**
   The Kind development cluster already hosts 10+ core components (Redpanda, Temporal, PostgreSQL+pgvector, Kong, Ollaya, Jaeger, Platform, Decision-Service, Mocks, AI-Runtime, and Operator). Adding MinIO, bucket initialization jobs, and S3 credentials adds pod lifecycle overhead and slows down local/CI feedback loops without altering the end-to-end RAG verification outcome.
2. **Acceptance Criteria Met:**
   The primary E2E workflow verifies RAG retrieval against `document_chunks` partitioned in PostgreSQL. The ingestion contract is fully demonstrated via Operator CRD reconciliation and Job lifecycle tracking.

---

## 3. Proposed Solution (When Addressed)

1. **Option A (Lightweight Ingestion Mocking - Recommended):**
   Add S3 protocol support (`boto3` / `aioboto3`) to `optimus_ai.rag.ingest` with an offline fallback or `moto` / `testcontainers-python` mock during integration tests.
2. **Option B (Optional MinIO Overlay for Full Emulation):**
   - Provide an optional Kustomize overlay in `deployments/overlays/minio-dev/` featuring a lightweight MinIO Deployment and a bucket seeding Job (`s3-seeder`).
   - Update `knowledge_source.yaml` in that overlay to point to `http://minio.optimus.svc:9000/acme-plm-docs/pumps`.

---

## 4. References & Related Documents

- [ADR-0003: Hybrid RAG Architecture - Operator Ingestion & MCP Delegation](../adr/0003-hybrid-rag-architecture-operator-ingestion-and-mcp-delegation.md)
- [`deployments/tenants/acme/knowledge_source.yaml`](../../deployments/tenants/acme/knowledge_source.yaml)
- [`projects/ai-runtime/src/optimus_ai/rag/ingest.py`](../../projects/ai-runtime/src/optimus_ai/rag/ingest.py)
