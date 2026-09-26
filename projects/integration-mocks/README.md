# Integration Mocks (`projects/integration-mocks`)

Enterprise mock servers simulating real-world operational systems (**EAM, PLM, ERP, FSM, OI**) exposing standard **Model Context Protocol (MCP)** tool interfaces for the AI Runtime and Platform workflows.

---

## 1. Overview & Architecture

To demonstrate realistic enterprise integration without depending on external vendor sandboxes (SAP, IFS Cloud, PTC Windchill, Maximo), `optimus` uses production-shaped mock services consolidated in a single Go binary with subcommands:

- **EAM (Enterprise Asset Management):** Asset master records, telemetry, maintenance and failure logs (`eam.get_asset`, `eam.get_maintenance_history`).
- **PLM (Product Lifecycle Management):** Engineering documentation, cooling manual search, technical specifications (`plm.search_documents`).
- **ERP (Enterprise Resource Planning):** Warehouse spare part stock checking, reservations, and Saga compensations (`erp.get_inventory`, `erp.reserve_inventory`, `erp.release_inventory_reservation`).
- **FSM (Field Service Management):** Dispatching work orders, technician scheduling (`fsm.create_work_order`).
- **OI (Operational Intelligence):** Real-time asset sensor telemetry and failure stream (`oi.get_recent_events`).

All tools are documented in declarative schemas located in `contracts/mcp/*.json`.

---

## 2. MCP Server Implementation & Tech Debt Notice

### Current State
`internal/mcpserver/server.go` currently implements a lightweight JSON-RPC 2.0 HTTP server supporting:
- `tools/list` (MCP tool discovery)
- `tools/call` (MCP tool execution)
- `GET /call-log` (deterministic call tracking used by E2E test assertions to verify that agents invoke expected tools)

### Modernization Roadmap (Official Go MCP SDK)
We have tracked the migration from our custom JSON-RPC transport to the **Official Anthropic Model Context Protocol Go SDK** (`github.com/modelcontextprotocol/go-sdk/mcp`) as a formal Technical Debt.

* **Tech Debt Document:** [TD-0001: Integration Mocks - Migration to Official Go MCP SDK](../../docs/tech-debts/TD-0001-official-go-mcp-sdk-migration.md)
* **Target:** Phase 9 (Hardening & Complete MCP 2024-11-05 spec with SSE transport).
* **Architecture Decision:** [ADR-0004: Dedicated MCP Adapters for Enterprise S2S Integrations](../../docs/adr/0004-dedicated-mcp-adapters-for-enterprise-s2s-integrations.md)

---

## 3. Running & Testing

### Running standalone
```bash
# Run all services or a specific domain
go run cmd/main.go serve eam --port 8080
go run cmd/main.go serve erp --port 8081
```

### Running Tests
```bash
go test ./... -v -race
```
