# AGENTS.md — optimus

This file is read automatically by AI coding assistants at the start of a session in this repository. It is the operating contract and the portable architecture/conventions reference; `docs/spec/PRD.md` is the full product spec. Read `docs/spec/PRD.md` before changing anything that touches an architectural boundary.

## What this project is, in one paragraph

`optimus` is a portfolio demo of an AI-native enterprise operations platform: enterprise system signals flow through investigation → RAG/knowledge retrieval → agent planning → a typed decision engine (Ollaya, SystemOne-API-compatible) → a durable Temporal workflow → a governed operational action, all inside a Kind Kubernetes cluster with a real Operator managing infrastructure lifecycle. See `docs/spec/PRD.md` §1 for the full purpose and design context, §4 for the scenarios, §7 for the Operator/CRD boundary — read that section before touching anything under `projects/operator`.

## Repository map (see docs/spec/PRD.md §6.3 for the authoritative full tree)

| Path | Language | Owns |
|---|---|---|
| `projects/platform` | Go | tenant, asset, work-order, integration APIs, Temporal worker, outbox |
| `projects/decision-service` | Go | Ollaya/SystemOne client, decision policy, decision audit |
| `projects/integration-mocks` | Go | EAM/PLM/ERP/FSM/OI mock servers + MCP server wrapper |
| `projects/operator` | Go | `EnterpriseEnvironment`, `EnterpriseIntegration`, `DecisionModel`, `DecisionPolicy`, `EnterpriseKnowledgeSource` CRDs + controllers |
| `projects/ai-runtime` | Python | planner + specialist agents, RAG, MCP client, LLM provider abstraction |
| `projects/ollaya` | Go | deterministic local stub for the Ollaya decision engine — same SystemOne wire protocol, no GPU, no network |
| `e2e` | Go | Ginkgo/Gomega, real Kind cluster, one scenario per folder |
| `contracts` | OpenAPI/JSON | the only place cross-project types are defined |
| `deployments` | Kustomize | `base/` per component, `overlays/{kind-dev,kind-ci}`, `tenants/` |
| `docs/adr` | Markdown | ADRs — read the relevant one before changing an architectural boundary |
| `docs/stories`, `docs/workflows`, `docs/tech-debts` | Markdown | story specs, acceptance-test walkthroughs, tracked debt |
| `scripts` | Shell | cluster lifecycle, build/deploy, demos (wrapped by `mise` tasks) |

Each `projects/*` Go module is independently `go test`-able and independently deployable. `go.work` at the root is a **local-dev convenience only** — never assume it implies coupling. `ai-runtime` is intentionally outside `go.work` (it's Python, managed with `uv`).

## Non-negotiable boundaries (docs/spec/PRD.md §7, §9 — do not blur these without an explicit ADR)

1. **AI agents never touch a database, Temporal, or an enterprise system directly.** Always through MCP → `integration-mocks`, or through `decision-service` for typed decisions.
2. **The Kubernetes Operator manages infrastructure/application lifecycle only** (`EnterpriseEnvironment`, `EnterpriseIntegration`, `DecisionModel`). It never calls `decision-service`, never touches Temporal, never runs a business operation (`RunAgent`, `RunDecision`, `CreateWorkOrder` are explicitly **not** CRDs — see docs/spec/PRD.md §7.3).
3. **Temporal owns every durable, multi-step, human-approval-waiting business workflow.** It never reconciles Kubernetes resources.
4. **`decision-service` speaks the SystemOne wire protocol**, not an Ollaya-specific API — don't hardcode Ollaya-only fields into the shared decision schema (docs/spec/PRD.md §2.1, ADR‑003).
5. **Every tenant-owned row, tool call, and workflow carries `tenant_id`/`TenantContext`.** No new table, MCP tool, or activity ships without it.

If a task seems to require crossing one of these, stop and ask instead of finding a clever workaround.

## Commands

```bash
mise install && mise run bootstrap     # once, after clone
mise run kind-create                   # local Kind cluster + registry
mise run build                         # build+push all images to localhost:5001
mise run deploy                        # kustomize apply, dependency order
mise run test                          # fast tests only, no Kind — run this constantly
mise run lint                          # golangci-lint + ruff/mypy across all projects
mise run e2e                           # full Kind E2E suite (slow — run before declaring a phase "done")
mise run demo                          # the CLI summary demo

# inside a single Go project:
cd projects/<name> && go tool task test
cd projects/<name> && go tool task lint
```

## Definition of Done (docs/spec/PRD.md §15.2 — copy this checklist into your own summary when finishing a task)

- [ ] Lint + tests pass in every touched project (`go tool task lint && go tool task test`, or `ruff check . && mypy . && pytest`)
- [ ] Any changed `contracts/{openapi,proto,mcp}` file has its generated code regenerated and committed
- [ ] New tables/queries went through a `goose` migration + `sqlc` regeneration — never hand-written SQL in application code
- [ ] Tenant context threaded through anything new
- [ ] New/changed CRD types have matching `config/crd` YAML + a `config/samples/` example
- [ ] A relevant ADR added/updated if an architectural decision was actually made
- [ ] If the change touches the primary demo path, `e2e/scenarios/asset-failure` still passes
- [ ] If the change adds/removes a container or moves a responsibility across an architectural boundary, the relevant diagram(s) in `docs/spec/PRD.md` §18 are updated in the same PR

## Coding conventions

- **Go**: layered internal packages — `internal/app` (use-cases), `internal/domain` (entities/interfaces), `internal/infra` (adapters), `internal/transport` (HTTP/handlers). `sqlc` + `goose` + `pgx/v5`, never an ORM. `golangci-lint` config at the repo root, each project's `.golangci.yml` extends it. Table-driven tests, `testify` assertions, `-race` always on in `go tool task test`.
- **Python**: `uv`-managed, `ruff` + `mypy`/`pyright` clean, Pydantic AI for orchestration, async SQLAlchemy 2.0 for pgvector access. Type hints are mandatory, not optional, given Python 3.14's improved annotation semantics.
- **Kubernetes types**: generate with `kubebuilder`/`controller-gen`, never hand-write CRD YAML — hand-written CRD YAML and generated Go types drift.
- **Tests default to structural assertions.** Anywhere an LLM or agent is in the loop, assert on typed fields, tool-call logs, or decision-service responses — never on substrings of generated prose (docs/spec/PRD.md §8.3, §11.6, ADR‑011).

## When you're unsure

Prefer re-reading `docs/spec/PRD.md` §7 (Operator/CRD boundary) or §9 (Temporal/events/security boundary) over guessing. If the PRD genuinely doesn't answer the question, add it to `docs/spec/PRD.md` §16 (Open Questions) rather than silently picking an answer and moving on.
