# decision-service

**Governed decision engine.** Receives an `EvidenceContext` from the Temporal workflow, forwards it to Ollaya via the SystemOne wire protocol, evaluates the raw probabilistic response against deterministic policy rules, and returns a versioned, auditable `GovernedDecision`.

> **Boundary:** This service has no knowledge of `ai-runtime`. It is called directly over HTTP by the platform Temporal worker — not by AI agents.

## Architecture overview

```
Temporal Worker (platform)
        │  POST /v1/decisions
        ▼
┌──────────────────────────────────────────┐
│  HTTP Handler                            │
│  internal/http/handler.go                │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │  decision.Service                  │  │
│  │                                    │  │
│  │  1. Build question set             │  │
│  │     severity   (score/0..3)        │  │
│  │     safety_risk (choice)           │  │
│  │     field_visit_required (noul)    │  │
│  │                                    │  │
│  │  2. systemone.Client               │  │
│  │     POST /v1/systemone → Ollaya    │  │
│  │                                    │  │
│  │  3. policy.Engine (policy_v1)      │  │
│  │     Rule A: safety_risk=HIGH       │  │
│  │             → approval required    │  │
│  │     Rule B: P1 + field_visit       │  │
│  │             → supervisor approval  │  │
│  │     Rule C: confidence < 0.75      │  │
│  │             → human verification   │  │
│  │                                    │  │
│  │  4. audit.Store → Postgres (RLS)   │  │
│  └────────────────────────────────────┘  │
│                                          │
│  Response: GovernedDecision JSON         │
└──────────────────────────────────────────┘
                 │
                 ▼
        ┌────────────────┐
        │  Ollaya        │  ← port 11435
        │  /v1/systemone │  ← SystemOne wire protocol
        │  /api/pull     │  ← model lifecycle
        └────────────────┘
```

## Package structure

```
projects/decision-service/
├── cmd/
│   └── main.go               # Wire: OTel → audit.Store → client → policy.Engine → svc → handler
├── internal/
│   ├── systemone/
│   │   └── client.go         # SystemOne HTTP client — W3C traceparent injection, 10s timeout
│   ├── decision/
│   │   └── service.go        # Question building, Ollaya call, stub fallback
│   ├── policy/
│   │   └── engine.go         # Deterministic rule evaluation — policy_v1
│   ├── audit/
│   │   ├── audit.go          # Record type + Store interface + MemoryStore
│   │   └── postgres.go       # PostgresStore — pgx/v5, RLS, JSONB
│   ├── http/
│   │   └── handler.go        # GET /healthz · POST /v1/decisions
│   └── telemetry/
│       └── tracer.go         # OTel TracerProvider init (OTLP gRPC + Prometheus)
├── db/
│   └── migrations/
│       └── 00001_init_decision_audit.sql  # decision_audit table + RLS policy + index
├── Dockerfile                # golang:1.24-alpine → alpine:3.20, port 8082
└── go.mod                    # module github.com/optimus/projects/decision-service, Go 1.24
```

## HTTP API

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check |
| `POST` | `/v1/decisions` | Produce a governed decision based on evidence context |
| `GET` | `/metrics` | Prometheus metrics |

### Decision request

```bash
curl -X POST http://localhost:8082/v1/decisions \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: acme" \
  -d '{
    "tenant_id": "acme",
    "asset_id": "P-104",
    "asset_type": "Centrifugal Pump",
    "failures_last_30_days": 4,
    "plm_findings": "PLM-COOL-4021 §4.2: Known cooling-system failure mode",
    "spare_part_available": true
  }'
```

### Response — `GovernedDecision`

```json
{
  "decision_id": "dec-550e8400-...",
  "asset_id": "P-104",
  "tenant_id": "acme",
  "severity": "P1",
  "safety_risk": "HIGH",
  "field_visit_required": true,
  "requires_approval": true,
  "approval_reason": "SafetyRisk=HIGH requires supervisory approval per policy_v1",
  "confidence": 0.94,
  "policy_version": "policy_v1",
  "created_at": "2026-09-27T10:00:00Z"
}
```

## SystemOne wire protocol

`decision-service` calls Ollaya **not directly** — but via the SystemOne wire format. This means a real TypeSafe/SystemOne endpoint can be substituted for Ollaya by simply changing an environment variable (ADR-003).

```jsonc
// POST /v1/systemone → Ollaya (or a real TypeSafe endpoint)
{
  "model": "laya",
  "state": {
    "asset_id": "P-104",
    "failures_last_30_days": 4,
    "plm_findings": "...",
    "spare_part_in_stock": true
  },
  "questions": {
    "severity": {
      "type": "score",
      "instructions": "How severe is this asset failure?",
      "criteria": ["Low — monitor", "Medium — schedule", "High — P1 immediate"]
    },
    "safety_risk": {
      "type": "choice",
      "criteria": {"low": "No safety implication", "medium": "Possible", "high": "Immediate risk"}
    },
    "field_visit_required": {
      "type": "noul",
      "criteria": {"true": "On-site visit required", "false": "Can be handled remotely"}
    }
  }
}
```

Ollaya returns structured probabilistic responses in a **single forward pass** — it does not perform token-by-token text generation. This allows `decision-service` E2E tests to make structural assertions on `severity`, `safety_risk`, and `field_visit_required` fields without inspecting text output.

## Policy engine — policy_v1

`policy/engine.go` applies deterministic rules — probabilistic Ollaya output passes through here:

```
Rule A: safety_risk == "HIGH"
         → requires_approval = true
         → reason: "SafetyRisk=HIGH requires supervisory approval per policy_v1"

Rule B: severity >= P1 AND field_visit_required == true
         → requires_approval = true
         → reason: "P1 severity with field visit requires supervisory confirmation"

Rule C: confidence < 0.75
         → requires_approval = true
         → reason: "Low confidence requires human verification"

Mapping: Ollaya score → Severity
         score >= 2.0  → P1
         score >= 1.0  → P2
         score <  1.0  → P3
```

The confidence threshold (0.75) and rule list are config-driven — no code changes required.

## Database schema

```sql
-- 00001_init_decision_audit.sql
CREATE TABLE decision_audit (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      TEXT NOT NULL,
    asset_id       TEXT NOT NULL,
    severity       TEXT NOT NULL,
    safety_risk    TEXT NOT NULL,
    field_visit    BOOLEAN NOT NULL,
    requires_approval BOOLEAN NOT NULL,
    confidence     FLOAT8 NOT NULL,
    policy_version TEXT NOT NULL,
    raw_response   JSONB,           -- raw Ollaya response
    governed_decision JSONB,        -- GovernedDecision struct
    created_at     TIMESTAMPTZ DEFAULT NOW()
);

-- RLS policy — tenant isolation
ALTER TABLE decision_audit ENABLE ROW LEVEL SECURITY;
CREATE POLICY decision_audit_tenant_isolation ON decision_audit
    USING (tenant_id = current_setting('app.current_tenant'));

-- Composite index
CREATE INDEX ON decision_audit (tenant_id, asset_id, created_at DESC);
```

Every `PostgresStore` query first runs `set_config('app.current_tenant', $tenant_id, true)`; the RLS policy filters out records belonging to other tenants.

## Stub fallback

If Ollaya is unreachable (CI, development, connection refused), `decision.Service` returns a deterministic stub response:

```go
// severity: HIGH (score: 2.1), safety_risk: HIGH, field_visit_required: true, confidence: 0.94
```

This allows `decision-service` unit tests to run without an Ollaya dependency.

## Dependencies

| Dependency | Version | Purpose |
|---|---|---|
| `jackc/pgx/v5` | v5.x | PostgreSQL driver |
| `opentelemetry-go` | v1.31.0 | OTel tracing + metrics |
| `prometheus/client_golang` | v1.20.5 | /metrics endpoint |

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8082` | HTTP listening port |
| `DATABASE_URL` | — | PostgreSQL connection string (optional — falls back to MemoryStore) |
| `OLLAYA_URL` | — | Ollaya endpoint (`http://ollaya.svc:11435`) |
| `DECISION_MODEL` | `laya` | Ollaya model name |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `jaeger.optimus.svc:4317` | OTel gRPC target |

## Local development

```bash
# Download dependencies
go mod download

# Apply migrations
goose -dir db/migrations postgres "$DATABASE_URL" up

# Run tests
go test -race ./...

# Start the service (with stub fallback, no Ollaya required)
go run ./cmd/main.go -port 8082
```

## Build

```bash
docker build -t localhost:5001/optimus/decision-service:dev .
docker push localhost:5001/optimus/decision-service:dev
```

## Related components

| Component | Relationship |
|---|---|
| [`platform`](../platform/README.md) | The `RunDecision` Temporal activity calls this service |
| [`ollaya`](../ollaya/README.md) | Performs decision inference (SystemOne protocol) |
