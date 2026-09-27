# integration-mocks

A single-binary Go mock server that simulates five enterprise systems (**EAM, PLM, ERP, FSM, OI**). Each system is exposed as MCP (JSON-RPC 2.0 over HTTP) tools. `ai-runtime` agents and `platform` Temporal activities consume this service as if it were a real enterprise system.

## Responsibility

```
ai-runtime   (MCPClient)     ─┐
platform     (MCP activity)  ─┼──► POST / (JSON-RPC 2.0) ──► integration-mocks
operator     (health check)  ─┘                                    │
                                                                    ├─ eam.*
                                                                    ├─ plm.*
                                                                    ├─ erp.*
                                                                    ├─ fsm.*
                                                                    └─ oi.*
```

E2E tests structurally verify which tools were called and with which arguments via the `GET /call-log` endpoint — LLM output text is never inspected (ADR-011).

## Package structure

```
projects/integration-mocks/
├── cmd/
│   └── main.go              # -system and -port flags, OTel init, domain Register() calls
├── internal/
│   ├── mcpserver/
│   │   ├── server.go        # JSON-RPC 2.0 HTTP server, tools/list, tools/call, /call-log
│   │   └── server_test.go
│   ├── eam/
│   │   └── service.go       # eam.get_asset, eam.get_maintenance_history
│   ├── plm/
│   │   └── service.go       # plm.search_documents
│   ├── erp/
│   │   └── service.go       # erp.get_inventory, erp.reserve_inventory, erp.release_inventory_reservation
│   ├── fsm/
│   │   └── service.go       # fsm.create_work_order
│   ├── oi/
│   │   └── service.go       # oi.get_recent_events
│   └── telemetry/
│       └── tracer.go        # OTel init — OTLP gRPC, W3C TraceContext propagator
├── Dockerfile                # golang:1.24-alpine → alpine:3.20, port 8080
└── go.mod                    # module github.com/optimus/projects/integration-mocks, Go 1.24
```

## Startup options

```bash
# Enable all systems (default)
integration-mocks --system=all --port=8080

# EAM + PLM only
integration-mocks --system=eam,plm --port=8080

# Single system
integration-mocks --system=fsm --port=8083
```

## MCP protocol

All requests arrive at `POST /` in JSON-RPC 2.0 format:

```bash
# Tool discovery
curl -X POST http://localhost:8080/ \
  -d '{"jsonrpc":"2.0","method":"tools/list","id":1}'

# Tool invocation
curl -X POST http://localhost:8080/ \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
      "name": "eam.get_maintenance_history",
      "arguments": {"tenant_id": "acme", "asset_id": "P-104"}
    },
    "id": 2
  }'

# E2E verification — call log
curl http://localhost:8080/call-log
```

## Tool catalogue

### EAM — Enterprise Asset Management

#### `eam.get_asset`
Returns asset master data.

```json
// Input
{"tenant_id": "acme", "asset_id": "P-104"}

// Output
{
  "asset_id": "P-104",
  "name": "Centrifugal Pump P-104",
  "type": "Centrifugal Pump",
  "plant": "Manchester",
  "status": "WARNING",
  "criticality": "HIGH"
}
```

#### `eam.get_maintenance_history`
Maintenance history and failure count for the last 30 days.

```json
// Output
{
  "asset_id": "P-104",
  "failures_last_30_days": 4,
  "events": [
    {"date": "2026-08-30", "event": "overheating", "resolved": true},
    {"date": "2026-09-06", "event": "overheating", "resolved": true},
    {"date": "2026-09-14", "event": "overheating", "resolved": true},
    {"date": "2026-09-22", "event": "overheating", "resolved": false}
  ]
}
```

---

### PLM — Product Lifecycle Management

#### `plm.search_documents`
Hybrid maintenance manual search. Queries containing `overheating`, `cooling`, or `p-104` return PLM-COOL-4021.

```json
// Input
{"tenant_id": "acme", "query": "P-104 cooling overheating"}

// Output
{
  "results": [{
    "doc_id": "PLM-COOL-4021",
    "title": "Centrifugal Pump Cooling Loop Maintenance Manual",
    "section": "§4.2 — Known Cooling-System Failure Mode",
    "score": 0.92,
    "spare_part": "SP-COOL-9981"
  }]
}
```

---

### ERP — Enterprise Resource Planning

#### `erp.get_inventory`
Real-time inventory status.

```json
// Input
{"tenant_id": "acme", "part_id": "SP-COOL-9981"}

// Output
{"part_id": "SP-COOL-9981", "in_stock": 6, "warehouse": "Manchester-WH1", "lead_days": 0}
```

#### `erp.reserve_inventory`
Idempotent inventory reservation. Repeated calls with the same `idempotency_key` return the same `reservation_id`.

```json
// Input
{"tenant_id": "acme", "part_id": "SP-COOL-9981", "quantity": 1, "idempotency_key": "wf-run-id"}

// Output
{"reservation_id": "RES-9981-...", "status": "RESERVED", "idempotent": false}
```

#### `erp.release_inventory_reservation`
Reservation cancellation for saga compensation.

```json
// Input
{"tenant_id": "acme", "reservation_id": "RES-9981-..."}

// Output
{"reservation_id": "RES-9981-...", "status": "RELEASED", "compensated": true}
```

---

### FSM — Field Service Management

#### `fsm.create_work_order`
Idempotent work order creation. The Temporal `WorkflowID` is used as the idempotency key.

```json
// Input
{"tenant_id": "acme", "asset_id": "P-104", "priority": "P1", "idempotency_key": "wf-run-id"}

// Output
{
  "work_order_id": "WO-P-104-10423",
  "status": "ASSIGNED",
  "assigned_tech": "John Smith",
  "idempotent": false
}
```

---

### OI — Operational Intelligence

#### `oi.get_recent_events`
Real-time telemetry data.

```json
// Input
{"tenant_id": "acme", "asset_id": "P-104"}

// Output
{
  "asset_id": "P-104",
  "telemetry": {
    "temperature_celsius": 98.4,
    "vibration_rms": 4.2,
    "flow_rate_lpm": 142.0,
    "status": "WARNING_CRITICAL_TEMPERATURE",
    "recorded_at": "2026-09-27T10:00:00Z"
  }
}
```

---

## Idempotency

ERP and FSM services provide idempotency via in-memory maps protected by `sync.Mutex`:

```go
// Second call with the same idempotency_key
{"reservation_id": "RES-9981-...", "status": "RESERVED", "idempotent": true}
```

This makes it safe for a tool call to occur twice during Temporal retry scenarios.

## E2E call log

```bash
# Fetch all tool calls for a specific asset
curl "http://localhost:8080/call-log?asset_id=P-104"

# Example response
[
  {"tool": "eam.get_maintenance_history", "args": {"asset_id": "P-104", "tenant_id": "acme"}},
  {"tool": "plm.search_documents",        "args": {"query": "P-104 cooling overheating", "tenant_id": "acme"}},
  {"tool": "erp.get_inventory",           "args": {"part_id": "SP-COOL-9981", "tenant_id": "acme"}},
  {"tool": "erp.reserve_inventory",       "args": {"idempotency_key": "wf-...", ...}},
  {"tool": "fsm.create_work_order",       "args": {"idempotency_key": "wf-...", ...}}
]
```

Ginkgo E2E tests assert against this log using the `HaveMCPCall` matcher.

## Observability

- An OTel span is produced from the W3C `traceparent` header for each tool call
- Span name: `tools/call.<tool_name>` (e.g. `tools/call.fsm.create_work_order`)
- The full tool call chain platform → ai-runtime → integration-mocks is visible in Jaeger
- The `/metrics` endpoint exposes per-tool call counts and latency histograms

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port (also configurable via -port flag) |
| `SYSTEM` | `all` | Systems to enable (also configurable via -system flag) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `jaeger.optimus.svc:4317` | OTel gRPC target |
| `ENVIRONMENT` | `dev` | OTel resource attribute |

## Local development

```bash
go mod download
go test -race ./...

# Start all mock systems
go run ./cmd/main.go --system=all --port=8080
```

## Build

```bash
docker build -t localhost:5001/optimus/integration-mocks:dev .
docker push localhost:5001/optimus/integration-mocks:dev
```

## Technical debt

- **TD-0001:** MCP transport is hand-rolled JSON-RPC 2.0 over HTTP. Migration to the official Go MCP SDK is planned.

## Related components

| Component | Relationship |
|---|---|
| [`ai-runtime`](../ai-runtime/README.md) | Makes tool calls via MCPClient |
| [`platform`](../platform/README.md) | ERP/FSM Temporal activities are invoked over MCP |
| [`operator`](../operator/README.md) | `EnterpriseIntegration` CRD provisions these mocks |
