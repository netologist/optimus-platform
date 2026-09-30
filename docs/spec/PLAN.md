# `optimus` — Master Implementation Plan & Progress Tracking

This document tracks the end-to-end phases of the `optimus` platform, the deliverable steps within each phase, their dependencies, and their completion status.

---

## Overall Progress Summary

- [x] **Phase 0: Documentation & Architecture Baselining** (Completed ✅)
- [x] **Phase 1: Foundation (Mise, Monorepo, Kind, Postgres, CI)** (Completed ✅)
- [x] **Phase 2: Kubernetes Operator (CRDs & Controllers)** (Completed ✅)
- [x] **Phase 3: Enterprise Mocks & MCP Tool Manifests** (Completed ✅)
- [x] **Phase 4: Platform Core, Postgres RLS & Transactional Outbox** (Completed ✅)
- [x] **Phase 5: Decision Service (Ollaya SystemOne & Policy Engine)** (Completed ✅)
- [x] **Phase 6: AI Runtime (Pydantic AI, Dynamic MCP, Hybrid RAG)** (Completed ✅)
- [x] **Phase 7: Temporal Workflows, Saga Compensation & HITL Approval** (Completed ✅)
- [x] **Phase 8: Kind E2E Integration Suite & Interactive Demo** (Completed ✅)
- [x] **Phase 9: Multi-Tenant Extensibility, Declarative Skills & Operator Knowledge Ingestion** (Completed ✅)
---

## Phase Details and Completion Status

### Phase 0: Documentation & Architecture Baselining
- [x] Clarified 12 architectural decisions through a grilling session.
- [x] Baselined the architecture decision record set with diagrams.
- [x] Updated the `PRD.md` document (Saga, Outbox, Trace, Dynamic MCP, RLS, GovernedDecision).
- [x] Created the master implementation plan, `PLAN.md`.

### Phase 1: Foundation
- [x] `.mise.toml` configuration (Go 1.26/1.27, Python 3.14.x, kubectl, kind, kustomize, buf, sqlc, goose, lefthook).
- [x] Root `go.work` and module skeletons (`projects/platform`, `projects/decision-service`, `projects/integration-mocks`, `projects/operator`, `e2e`).
- [x] Setup of `.golangci.yml` and `.editorconfig`, `.dockerignore`, `lefthook.yml`.
- [x] `deployments/kind/cluster.yaml` and `scripts/kind-create.sh`, `scripts/kind-destroy.sh` (with a local registry mirror).
- [x] PostgreSQL 17 + `pgvector` Kustomize deployment and `goose` migration templates.
- [x] GitHub Actions CI skeleton (`.github/workflows/ci.yaml`).

### Phase 2: Kubernetes Operator
- [x] kubebuilder skeleton under `projects/operator` (`platform.optimus.dev/v1alpha1`).
- [x] CRD types and YAML manifests:
  - `EnterpriseEnvironment` (tenant demo meta-resource).
  - `EnterpriseIntegration` (EAM/PLM/ERP/FSM/OI mock pod management).
  - `DecisionModel` (Ollaya model pull & warm management).
- [x] Controller reconciliation logic and `metav1.Condition` status management.
- [x] Controller unit tests (fast-running tests with a fake client).
- [x] Declarative samples under `config/samples/`.

### Phase 3: Enterprise Mocks & MCP Tool Manifests
- [x] `contracts/mcp/*.json` tool manifests (`eam.get_asset`, `eam.get_maintenance_history`, `plm.search_documents`, `erp.get_inventory`, `erp.reserve_inventory`, `erp.release_inventory_reservation`, `fsm.create_work_order`, `oi.get_recent_events`).
- [x] `contracts/openapi/platform.yaml` and `contracts/openapi/decision.yaml`.
- [x] `projects/integration-mocks` Go module:
  - JSON-RPC 2.0 / MCP HTTP server (`internal/mcpserver`).
  - EAM, PLM, ERP, FSM, OI subsystems.
  - Asset failure history and inventory fixture data.

### Phase 4: Platform Core, Postgres RLS & Transactional Outbox
- [x] `projects/platform` domain entities (`Tenant`, `Asset`, `Signal`, `WorkOrder`, `OutboxMessage`).
- [x] PostgreSQL 17 Row-Level Security (RLS) migrations (`SET LOCAL app.current_tenant`).
- [x] Go `TenantContext` and HTTP middleware.
- [x] HTTP transport layer (`POST /v1/tenants/{id}/signals`, `GET /v1/tenants/{id}/tools`, `POST /v1/tenants/{id}/approvals/{id}/approve`).
- [x] Transactional Outbox table and the `internal/infra/outbox/relay.go` Redpanda producer.
- [x] RLS and outbox integration tests.

### Phase 5: Decision Service & Ollaya Integration
- [x] `projects/decision-service` Go module.
- [x] SystemOne wire protocol HTTP client (Ollaya or TypeSafe compatible).
- [x] Deterministic Policy Engine (`policy_v1`):
  - `safety_risk == HIGH` -> `RequiresApproval: true`.
  - Confidence threshold configuration.
- [x] `decision_audit` table migration and repository (inference + applied policy version).
- [x] `POST /v1/decisions` HTTP endpoint and service unit tests.

### Phase 6: AI Runtime
- [x] `projects/ai-runtime` Python environment (`uv`, Python 3.14.x, Pydantic AI).
- [x] Dynamic MCP Client: fetches tenant tools from the Platform API and binds them to the agent session.
- [x] Planner Agent + Specialist Agents (EAM, PLM, ERP).
- [x] Hybrid RAG engine (`HybridRetriever`).
- [x] Dual LLM Provider abstraction (`MockProvider` for CI, `OllamaProvider`, `AnthropicProvider`).
- [x] `POST /investigate` HTTP server (`EvidenceContext` JSON output).
- [x] `pytest` unit tests (100% green in 0.34 seconds).

### Phase 7: Temporal Workflows, Saga Compensation & HITL Approval
- [x] `cmd/worker` Temporal Worker starter.
- [x] `AssetFailureWorkflow` Go definition:
  - Step 1: `InvestigateFailure` (AI Runtime call, 90s timeout, max 2 retries).
  - Step 2: `RunDecision` (Decision Service call).
  - Step 3: Human Approval Branch (waits 24h for the `approval-granted` signal if `RequiresApproval == true`).
  - Step 4: `ReserveSparePart` (ERP mock).
  - Step 5: `CreateFieldWorkOrder` (FSM mock).
- [x] Saga Compensation Mechanism: if the FSM step fails, `ReleaseSparePartReservation` is triggered (`FAILED_COMPENSATED`).
- [x] Full-scenario and Saga tests with the Temporal TestSuite.

### Phase 8: Kind E2E Integration Suite & Interactive Demo
- [x] `e2e` Ginkgo/Gomega test scenarios (`e2e/scenarios/asset-failure/`):
  - A pump P-104 failure signal is sent.
  - The agent's MCP calls are verified.
  - The typed decision output (`severity=P1`, `safety_risk=HIGH`) is verified.
  - The approval POST is submitted.
  - The FSM work order and call log are verified.
- [x] `scripts/demo.sh`: colorized CLI output, live agent progress, approval simulation, and a Jaeger trace link.
- [x] `scripts/build.sh`, `scripts/deploy.sh`, `scripts/cleanup.sh`.

### Phase 9: Multi-Tenant Extensibility, Declarative Skills & Operator Knowledge Ingestion
- [x] Created 7 new enterprise ADRs (`docs/adr/0001` - `0007`) and the `CONTEXT.md` glossary through grilling and domain modeling.
- [x] New CRD types and schemas (`projects/operator/api/v1alpha1/` and `projects/operator/config/crd/`):
  - `EnterpriseKnowledgeSource` (S3/SharePoint RAG synchronization).
  - `DecisionPolicy` (declarative risk and financial threshold rules).
  - `EnterpriseEnvironment` extension (`workflowRouting` and `skills` references).
- [x] Declarative Agent Skill Manifests (`contracts/skills/cooling_system_diagnostic.yaml`, `contracts/skills/inventory_replenishment.yaml`).
- [x] Declarative Decision Policy Manifests (`contracts/policies/manufacturing_standard_v1.yaml`, `defense_strict_v1.yaml`, `commercial_autonomous_v1.yaml`).
- [x] Updated the Acme tenant deployment package (`deployments/tenants/acme/`):
  - `knowledge_source.yaml`
  - `policy.yaml`
  - `environment.yaml` (with routing and capability definitions).
  - Successfully applied and verified against a live Kind cluster.
- [x] Prepared the `STORY-0002` specification document (`docs/stories/STORY-0002-tenant-workflow-routing-and-declarative-skills.md`).
