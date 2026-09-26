# ollaya

**Local Ollaya mock server.** A deterministic Go stub that replaces the real [ollaya.dev](https://ollaya.dev) decision inference engine. Runs with zero dependencies and zero GPU requirements in Kind clusters, CI, and local development.

> This project is not a real Ollaya/AI server. It returns hardcoded JSON responses. `decision-service` calls this endpoint over the SystemOne wire protocol.

## Why a stub?

| Requirement | Real Ollaya | This stub |
|---|---|---|
| GPU / CPU heavy computation | ✓ | ✗ (none) |
| Network dependency | ✓ (pull required) | ✗ |
| Deterministic CI responses | ✗ | ✓ |
| W3C TraceContext & Prometheus | ✓ | ✓ |
| SystemOne wire compatibility | ✓ | ✓ |
| Cold-start time in Kind | ~30-60s | <1s |

## Consumer map

| Consumer | How it uses this stub |
|---|---|
| `decision-service` | `POST /v1/systemone`, `/v1/decisions`, `/api/decide` |
| `operator` (DecisionModel reconciler) | `GET /` (health), `POST /api/pull` (model lifecycle) |
| Kubernetes manifests | `deployments/base/ollaya/` |
| CI/CD build scripts | `scripts/build.sh`, `build-and-test.yaml` |

## HTTP endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/` | Health check — `{"status": "ok"}` |
| `POST` | `/api/pull` | Model pull stub — `{"status": "success", "model": "<model>"}` |
| `POST` | `/v1/systemone` | SystemOne decision inference |
| `POST` | `/v1/decisions` | Same handler as `/v1/systemone` (alias) |
| `POST` | `/api/decide` | Ollaya native format (alias) |
| `GET` | `/metrics` | Prometheus metrics |

## SystemOne wire protocol

### Request format

```bash
curl -X POST http://localhost:11435/v1/systemone \
  -H "Content-Type: application/json" \
  -d '{
    "model": "laya",
    "state": {
      "asset_id": "P-104",
      "failures_last_30_days": 4,
      "plm_findings": "Known cooling-system failure mode PLM-COOL-4021 §4.2",
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
        "criteria": {"true": "On-site visit required", "false": "Remote resolution ok"}
      }
    }
  }'
```

### Fixed stub response

```json
{
  "model": "laya",
  "answers": {
    "severity": {
      "value": "High",
      "score": 2.1,
      "probability": 0.94
    },
    "safety_risk": {
      "value": "high",
      "score": 0.0,
      "probability": 0.94
    },
    "field_visit_required": {
      "value": true,
      "score": 0.0,
      "probability": 0.91
    }
  },
  "confidence": 0.94,
  "timing_ms": 18.5
}
```

This response triggers the `Rule A` condition in `decision-service/internal/policy/engine.go` (`safety_risk == HIGH → requires_approval = true`), causing E2E tests to exercise the approval workflow.

## Observability

Every `/v1/systemone` call (and its aliases) produces an OTel span named `SystemOneInference`:

```
Span: SystemOneInference
  attributes:
    model: laya
    confidence: 0.94
    timing_ms: 18.5
  parent: decision-service → RunDecision → Ollaya (W3C traceparent)
```

The full platform → decision-service → ollaya chain is visible in Jaeger.

- **OTel destination:** `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable (default: `jaeger.optimus.svc:4317`, gRPC)
- **Prometheus:** `/metrics` endpoint exposes request counts and a latency histogram

## Source structure

```
projects/ollaya/
├── main.go       # ~130 lines — HTTP mux, 5 routes, OTel init, Prometheus
├── go.mod        # module github.com/optimus/projects/ollaya, Go 1.24
├── go.sum
└── Dockerfile    # golang:1.24-alpine → alpine:3.20, EXPOSE 11435
```

All logic is kept in a single file (`main.go`) — this is an intentional design decision. The stub must not become complex; otherwise it becomes more expensive than real Ollaya.

## Dependencies

| Dependency | Version | Purpose |
|---|---|---|
| `prometheus/client_golang` | v1.20.5 | `/metrics` endpoint |
| `go.opentelemetry.io/otel` | v1.31.0 | OTel SDK |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | v1.31.0 | OTLP gRPC exporter |
| `google.golang.org/grpc` | v1.67.1 | gRPC transport |

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `11435` | HTTP listen port (matches real Ollaya) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `jaeger.optimus.svc:4317` | Jaeger gRPC endpoint |

## Running locally

```bash
go run ./main.go

# Health check
curl http://localhost:11435/

# Model pull (operator reconciler does this)
curl -X POST http://localhost:11435/api/pull \
  -d '{"model": "laya", "keep_alive": "-1"}'

# Decision inference
curl -X POST http://localhost:11435/v1/systemone \
  -H "Content-Type: application/json" \
  -d '{"model":"laya","state":{"asset_id":"P-104"},"questions":{}}'
```

## Build

```bash
docker build -t localhost:5001/ollaya-dev/ollaya:latest .
docker push localhost:5001/ollaya-dev/ollaya:latest
```

## Replacing with real Ollaya

Point the `OLLAYA_URL` environment variable in `decision-service` to a real Ollaya endpoint. No code changes required — the service speaks the SystemOne wire protocol and is provider-agnostic (ADR-003).

```bash
# Use real Ollaya
OLLAYA_URL=http://real-ollaya-host:11435 go run ./cmd/main.go
```

## Related components

| Component | Relationship |
|---|---|
| [`decision-service`](../decision-service/README.md) | Calls this stub via the SystemOne protocol |
| [`operator`](../operator/README.md) | `DecisionModel` reconciler uses `/` and `/api/pull` |
