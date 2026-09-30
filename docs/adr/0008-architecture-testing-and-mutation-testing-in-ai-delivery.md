# Architecture Testing and Targeted Mutation Testing in AI-Assisted Delivery

## Status
Accepted

## Context
With the adoption of AI-assisted engineering and automated agent code generation in `optimus`, velocity has shifted from manual keystrokes to specification synthesis, automated pull requests, and automated testing loops. However, AI agents exhibit well-known failure modes:
1. **Tautological & Shallow Test Synthesis**: Large language models frequently write regression tests that mirror implementation quirks, assert trivial state (e.g., `assert len(result) > 0`, `err != nil`), or pass without verifying behavioral invariants, driving high branch coverage without assertion efficacy.
2. **Architectural Boundary Erosion**: AI generators routinely violate strict layered dependencies unless mechanically blocked (e.g., calling domain models directly from transport, bypassing tenant context, leaking infrastructure into core logic, or violating the Operator/Temporal separation specified in `AGENTS.md`).

Standard unit test suites and line coverage metrics do not reliably catch these failure modes. Architectural fitness functions and mutation testing evaluate whether code adheres to systemic boundaries and whether tests actually detect injected defects.

## Decision

### 1. Enforce Architectural Fitness Functions (Architecture Tests)
We enforce compile/test-time architectural boundary testing across both Go and Python runtimes:
- **Go (`projects/platform`, `projects/decision-service`, `projects/operator`, `projects/integration-mocks`)**:
  - Implement architectural layer tests via Go AST / dependency analysis (using tools like `arch-go` or Go test suites inspecting `go/packages` dependency graphs).
  - Invariants asserted:
    - `internal/domain` cannot import `internal/infra`, `internal/app`, or `internal/transport`.
    - `internal/app` can import `internal/domain`, but cannot import `internal/transport`.
    - No package under `projects/operator` may import `temporal` or `projects/decision-service`.
    - No package under `projects/platform` or `projects/decision-service` may bypass `tenant.TenantContext` in repository and activity contracts.
- **Python (`projects/ai-runtime`)**:
  - Enforce module import boundaries using `pytest-archon` or AST import linting rules.
  - Invariants asserted:
    - Core agent logic cannot import concrete HTTP transport directly.
    - Specialized tools must access enterprise data via MCP abstractions, never direct database drivers.

Architecture tests execute on every local test run (`go tool task test`, `pytest`) and as a strict gate on every PR in CI.

### 2. Introduce Targeted Mutation Testing in Asynchronous/Nightly Pipelines
Mutation testing evaluates test quality by deliberately injecting synthetic faults (mutations) into AST nodes and verifying that tests "kill" the mutants:
- **Go**: We adopt `go-mutesting` / `gremlins`.
- **Python**: We adopt `mutmut`.
- **Execution Strategy**:
  - Full mutation testing has exponential complexity ($O(N \times M)$ where $N$ is mutants and $M$ is test run duration). Running it globally across all modules on every commit would severely degrade developer feedback loops.
  - **PR Gates (Incremental/Targeted)**: Mutation runs are scoped strictly to changed critical business packages (e.g., `internal/domain`, `internal/app/policy`, `src/optimus_ai/orchestration`).
  - **Scheduled/Nightly CI**: Full mutation evaluation runs on a scheduled cadence against `main`, tracking the Mutation Score Indicator (MSI) to prevent test regression.

## Consequences

### Positive
- **Guaranteed Boundary Invariants**: Architectural degradation caused by fast-moving human or LLM code generation is flagged instantly at build time without relying solely on manual PR reviews.
- **Resilience Against Vacuous LLM Tests**: Tests that generate false confidence through meaningless assertions are exposed as surviving mutants.
- **Clear Engineering Signal**: Provides concrete metrics (MSI and zero architecture violations) to benchmark test quality in an AI-native codebase.

### Negative / Trade-offs
- **Execution Overhead**: Mutation testing requires CPU time and caching discipline; must be carefully partitioned into PR-targeted runs and nightly suites to avoid slowing CI.
- **Maintenance of Architectural Rules**: As new internal packages and boundaries are introduced, architecture test manifests/specs must be maintained.
