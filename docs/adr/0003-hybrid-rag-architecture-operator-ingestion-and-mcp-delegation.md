# Hybrid Knowledge Retrieval: Operator-Managed Ingestion and Federated MCP Search

## Status
Accepted

## Context
Enterprise asset operations require accessing vast unstructured technical documentation (50+ GB PDF manuals, maintenance bulletins, engineering schematics) stored across heterogeneous sources (AWS S3, Microsoft SharePoint, on-premise shares) or pre-indexed within existing vendor platforms (Siemens Teamcenter, IFS Cloud Document Management). Exposing a synchronous HTTP document upload API is impractical for enterprise-scale corpora, while relying exclusively on local vector databases duplicates data when modern enterprise search APIs already exist.

## Decision
We adopt a hybrid knowledge retrieval architecture:
1. **Operator-Managed Ingestion (`EnterpriseKnowledgeSource` CRD):** For raw document repositories (S3, SharePoint, SMB), a custom Kubernetes resource specifies storage location, credentials, chunking parameters, and sync schedule. The Kubernetes Operator reconciles this by managing an ingestion CronJob that populates tenant-partitioned PostgreSQL `pgvector` tables.
2. **Federated MCP Search Delegation:** When an enterprise system already maintains a search/retrieval engine, `ai-runtime` delegates directly to the system's search capability via an MCP tool call (`plm.search_documents`) rather than copying data into local vector stores.
3. The AI Runtime's hybrid retriever merges local vector/keyword results with federated MCP search responses.

## Consequences
- Operator handles infrastructure provisioning (storage, secrets, CronJob scheduling) without touching runtime search queries.
- Avoids building fragile synchronous HTTP file upload endpoints.
- Respects existing enterprise data investments by querying native PLM/EAM search APIs where available.
- Enforces multi-tenant data boundaries at both the Kubernetes namespace and PostgreSQL RLS levels.
