# TD-0001: Integration Mocks - Migration to Official Go MCP SDK (`github.com/modelcontextprotocol/go-sdk/mcp`)

- **Status:** Open / Accepted Tech Debt
- **Impact Area:** `projects/integration-mocks` (`internal/mcpserver/`), `projects/ai-runtime` (`src/optimus_ai/mcp/`)
- **Target Release:** Phase 9 (Production-Quality Engineering / Enterprise Hardening)
- **Component Owner:** Platform & Integration Engineering

---

## 1. Problem & Current State

Currently, `projects/integration-mocks/internal/mcpserver/server.go` implements a lightweight, custom JSON-RPC 2.0 HTTP server supporting:
- `tools/list`
- `tools/call`
- Custom test inspection endpoint: `GET /call-log`

While this minimal implementation provides deterministic responses and fast test execution for local Kind cluster and CI environments, it:
1. Does not fully implement the complete MCP 2024-11-05 specification (e.g., standard `initialize`, `notifications/initialized`, `ping`, protocol version negotiation, client capabilities).
2. Uses raw HTTP POST JSON-RPC instead of standard MCP transports (SSE - Server-Sent Events or Stdio).
3. Requires hand-rolled JSON schema validation and error wrapping rather than leveraging standard SDK primitives.

---

## 2. Proposed Solution

Migrate `projects/integration-mocks` to use the official Anthropic Go SDK:
```go
import "github.com/modelcontextprotocol/go-sdk/mcp"
```

### Key Implementation Aspects:
1. **Server Instantiation:**
   Replace custom `mcpserver.Server` struct with `mcp.NewServer(...)` configured with enterprise tool capabilities.
2. **Tool Handlers:**
   Refactor service packages (`eam`, `plm`, `erp`, `fsm`, `oi`) to register tools using typed SDK handlers:
   ```go
   func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)
   ```
3. **Preserving `/call-log` for E2E Tests:**
   Maintain an audit/interceptor wrapper around the SDK handler or an HTTP mux route so E2E tests (`e2e/helpers/client.go`) can continue asserting structural tool calls (`GET /call-log`).
4. **Transport Compatibility:**
   Expose both SSE transport (`/sse`) and Streamable HTTP POST to ensure seamless connectivity from both Python MCP client (`fastmcp` / official `mcp` client) and future Go-based agents.

---

## 3. Why Deferred as Tech Debt?

- The current custom implementation fulfills 100% of the Phase 1–8 acceptance criteria for the primary vertical demo (Pump P-104 asset failure, RAG, Ollaya decisioning, Temporal dispatch, and E2E verification).
- Refactoring the transport layer requires updating both the Go server and the Python MCP client simultaneously. Doing this as a dedicated modernization story prevents regressions during core demo stabilization.

---

## 4. References & Related Documents

- [PRD.md §6.3, §11.3](../../PRD.md) — Monorepo Architecture & MCP Manifests
- [ADR-0004: Dedicated MCP Adapters for Enterprise S2S Integrations](../adr/0004-dedicated-mcp-adapters-for-enterprise-s2s-integrations.md)
- [`projects/integration-mocks/README.md`](../../projects/integration-mocks/README.md)
