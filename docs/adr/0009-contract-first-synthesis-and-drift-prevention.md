# Contract-First Synthesis and Bidirectional Schema Drift Prevention

## Status
Accepted

## Context
In an AI-assisted engineering workflow, coding assistants and autonomous subagents frequently modify transport payloads, add ad-hoc response attributes, or introduce struct definitions directly in implementation code without updating upstream contract definitions. This introduces silent interface drift between decoupled services (`platform`, `decision-service`, `integration-mocks`, `ai-runtime`), breaking consumers, client generators, and MCP tool handlers.

Relying on manual code review to verify schema alignment fails when delivery cadence increases and agents produce large diffs. We require mechanical enforcement to guarantee that contracts remain the single source of truth across all languages and runtimes.

## Decision
1. **Single Source of Truth in `contracts/`**:
   - All HTTP REST interfaces must be authored as OpenAPI 3.1 specifications under `contracts/openapi/`.
   - All asynchronous domain event payloads and internal gRPC definitions must be defined as Protocol Buffers under `contracts/proto/`.
   - All tool capabilities consumed by LLM agents must be declared as machine-readable JSON schemas under `contracts/mcp/`.
   - All tenant decision policies and agent skills must be declared under `contracts/policies/` and `contracts/skills/`.

2. **Zero Hand-Written Cross-Service DTOs**:
   - Go server stubs, models, and clients must be generated strictly via `oapi-codegen` and `buf`. Hand-written payload structs mirroring wire contracts are forbidden.
   - Python Pydantic models for contracts must be generated or validated directly against the schemas.
   - MCP tool definitions across Go servers and Python clients must deserialize and validate against the same `contracts/mcp/*.json` specifications.

3. **CI Bidirectional Drift Enforcement**:
   - CI executes code generation targets (`buf generate`, OpenAPI generation, MCP schema checks) on every pull request.
   - Any uncommitted code diff (`git diff --exit-code`) fails the build immediately. If an agent modifies generated code directly without editing the contract, the generator overwrites it and flags the discrepancy.
   - Schema breaking changes are guarded by `buf breaking` against the `main` branch.

## Consequences

### Positive
- Prevents cross-service runtime regressions caused by agents silently altering payload formats.
- Simplifies agent prompting: agents are instructed to edit the contract schema first, run generation, and then fulfill the generated interface.
- Ensures documentation, mock servers, and production services remain perfectly synchronized.

### Negative / Trade-offs
- Modifying an API requires running generation tooling (`mise run bootstrap` or `go generate`) before implementation.
- Requires strict versioning discipline and occasional backward-compatibility shims when deprecating contract fields.
