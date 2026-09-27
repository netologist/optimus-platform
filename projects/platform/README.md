# platform

Enterprise-facing backend for the **optimus** AI-native operations platform. Ingests operational signals (asset failures, anomaly events), orchestrates multi-step Temporal workflows covering AI investigation → governed decision → human approval → field dispatch, and enforces multi-tenant isolation through PostgreSQL Row-Level Security.

## Responsibility boundaries

The platform maintains three architectural boundaries:

| Boundary | Rule |
|---|---|
| **AI calls** | No direct LLM/agent calls — the `ai-runtime` service is invoked over HTTP |
| **Decision** | Does not make business decisions itself — `decision-service` (SystemOne protocol) handles that |
| **Enterprise systems** | Does not connect directly to EAM/PLM/ERP/FSM/OI — communicates via MCP (`integration-mocks`) |

## Architecture overview

```
Kong API Gateway
      │
      ▼
┌─────────────────────┐
│  HTTP API (cmd/api) │  ← /v1/tenants/{id}/signals · /approvals · /tools
│  transport/         │
│  internal/tenant    │  ← TenantMiddleware, TenantContext
│  internal/app       │  ← IngestSignal, ApproveWorkflow, GetActiveTools
└────────┬────────────┘
         │ StartWorkflow / SignalApproval
         ▼
┌─────────────────────┐
│  Temporal Worker    │  ← cmd/worker
│  internal/workflow  │
│                     │  AssetFailureWorkflow
│  Activity 1:        │    → InvestigateFailure   (ai-runtime HTTP)
│  Activity 2:        │    → RunDecision           (decision-service HTTP)
│  Activity 3:        │    → ReserveSparePart      (MCP → ERP)
│  Approval wait:     │    → WaitForApproval       (Temporal Signal, 24h durable)
│  Activity 4:        │    → CreateFieldWorkOrder  (MCP → FSM)
│  Compensation:      │    → ReleaseSparePartReservation (Saga)
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  PostgreSQL 17      │  ← pgx/v5, sqlc, goose migrations, pgvector
│  + Row-Level Sec.   │  ← SET app.current_tenant per connection
│  Outbox table       │  ← domain events written atomically
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  Outbox Relay       │  ← cmd/outbox-relay
│                     │  Postgres → Redpanda (Kafka) at-least-once delivery
└─────────────────────┘
```

## Package structure

```
projects/platform/
├── cmd/
│   ├── api/              # HTTP server binary
│   ├── worker/           # Temporal worker binary
│   └── outbox-relay/     # Outbox → Redpanda relay binary
├── internal/
│   ├── app/              # Use-case layer: commands + queries
│   │   └── service.go    # IngestSignal, ApproveWorkflow, GetActiveTools
│   ├── domain/           # Domain entities and repository interfaces
│   │   └── models.go     # Signal, WorkOrder, OutboxMessage, Tool
│   ├── infra/            # Adapter layer
│   │   └── postgres/     # pgx pool, sqlc generated code, MemoryStorage
│   ├── transport/        # HTTP handler, middleware
│   │   ├── http_handler.go   # Route registration
│   │   └── openapi.json      # OpenAPI 3.0 spec (11 KB)
│   ├── workflow/         # Temporal workflow + activity definitions
│   │   ├── workflow.go   # AssetFailureWorkflow
│   │   └── activities/   # Each activity in a separate file
│   ├── tenant/           # TenantContext, TenantMiddleware, RLS helpers
│   └── audit/            # Audit logging
├── db/
│   ├── migrations/       # goose SQL migration files
│   └── queries/          # sqlc source .sql files
├── openapi/              # oapi-codegen generated stubs (gitignored)
├── sqlc.yaml             # sqlc config: pgx/v5, JSON tags
└── Dockerfile            # Multi-stage: golang:1.24-alpine → alpine:3.20
```

## HTTP API endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check |
| `GET` | `/openapi.json` | OpenAPI 3.0 spec |
| `GET` | `/docs` | Swagger UI |
| `POST` | `/v1/tenants/{id}/signals` | Create a new operational signal (starts a workflow) |
| `GET` | `/v1/tenants/{id}/tools` | List active MCP tools for a tenant |
| `POST` | `/v1/tenants/{id}/approvals/{workflow_id}/approve` | Approve a high-risk action |
| `POST` | `/v1/tenants/{id}/approvals/{workflow_id}/reject` | Reject an action |
| `GET` | `/metrics` | Prometheus metrics |

### Signal submission example

```bash
curl -X POST http://localhost:8080/v1/tenants/acme/signals \
  -H "Content-Type: application/json" \
  -d '{"asset_id": "P-104", "symptom": "repeated overheating"}'
# → {"workflow_id": "asset-failure-P-104-<uuid>", "status": "RUNNING"}
```

## Temporal workflow — AssetFailureWorkflow

```mermaid
flowchart TD
    A[Signal ingested] --> B[InvestigateFailure\nai-runtime → EAM+PLM+ERP]
    B --> C[RunDecision\ndecision-service → Ollaya]
    C --> D{RequiresApproval?}
    D -- yes --> E[WaitForApproval\nTemporal Signal, 24h timeout]
    E --> F[ReserveSparePart\nMCP → ERP]
    D -- no --> F
    F --> G[CreateFieldWorkOrder\nMCP → FSM]
    G --> H[WriteOutbox\nwork_order.created event]
    G -- FSM error --> I[ReleaseSparePartReservation\nSaga compensation]
```

Each activity is **idempotent** — the Temporal `WorkflowID` is used as the `idempotency_key`. Even if the Temporal worker restarts, the workflow resumes from where it left off.

## Multi-tenancy

Every HTTP request is intercepted by `tenant/middleware.go`:

1. `tenant_id` is extracted from the `X-Tenant-ID` header.
2. A `TenantContext` is created and passed through all layers (app → infra → workflow → activities).
3. Every Postgres query executes `set_config('app.current_tenant', $tenant_id, true)`.
4. PostgreSQL RLS policies (`USING tenant_id = current_setting('app.current_tenant')`) automatically filter out data from foreign tenants.

> **Rule:** Every new table, MCP tool, or Temporal activity must carry a `tenant_id` — no exceptions.

## Dependencies

| Dependency | Version | Purpose |
|---|---|---|
| `jackc/pgx/v5` | v5.x | PostgreSQL driver |
| `pgvector/pgvector-go` | latest | pgvector support |
| `go.temporal.io/sdk` | v1.29.1 | Temporal Go SDK |
| `opentelemetry-go` | v1.31.0 | OTel tracing + metrics |
| `prometheus/client_golang` | v1.20.5 | /metrics endpoint |
| `stretchr/testify` | v3.x | Test assertions |

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `DATABASE_URL` | — | `postgres://user:pass@host:5432/dbname` |
| `TEMPORAL_HOST` | `127.0.0.1:7233` | Temporal frontend address |
| `AI_RUNTIME_URL` | — | ai-runtime service URL |
| `DECISION_SERVICE_URL` | — | decision-service URL |
| `MCP_SERVER_URL` | — | integration-mocks MCP endpoint |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `jaeger.optimus.svc:4317` | OTel gRPC target |

## Local development

```bash
# Download dependencies
go mod download

# Apply migrations
goose -dir db/migrations postgres "$DATABASE_URL" up

# Regenerate sqlc
sqlc generate

# Run tests (with race detector)
go test -race ./...

# Start the API server
go run ./cmd/api

# Start the Temporal worker in a separate terminal
go run ./cmd/worker

# Start the outbox relay
go run ./cmd/outbox-relay
```

## Observability

- **Traces:** An OTel span with a W3C `traceparent` header is produced for every HTTP request, Temporal activity, and Postgres query. Viewable in Jaeger.
- **Metrics:** Prometheus metrics over the `/metrics` endpoint: request counts, latency histograms, workflow states.
- **Logs:** Structured JSON logging via `slog`; every log line includes `trace_id` and `tenant_id`.

## Build

```bash
# Docker image (contains both api + worker binaries)
docker build -t localhost:5001/optimus/platform:dev .

# Push to Kind cluster
docker push localhost:5001/optimus/platform:dev
```

The image contains two binaries:
- `/app/platform-api` — default entrypoint
- `/app/platform-worker` — runs as a separate pod in the Kubernetes Deployment

## Related components

| Component | Relationship |
|---|---|
| [`decision-service`](../decision-service/README.md) | Workflow calls it via the `RunDecision` activity |
| [`ai-runtime`](../ai-runtime/README.md) | Workflow calls it via the `InvestigateFailure` activity |
| [`integration-mocks`](../integration-mocks/README.md) | ERP/FSM tools are called via MCP |
| [`operator`](../operator/README.md) | `EnterpriseEnvironment` CRD provisions the platform |
