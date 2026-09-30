# Automated Invariant and Property-Based Testing for AI-Generated Business Logic

## Status
Accepted

## Context
When task prompts instruct AI coding assistants to author unit tests, LLMs exhibit a systematic bias toward "happy-path" verification. They generate sample inputs mirroring internal implementation code and assert trivial outcomes. Boundary conditions—such as extreme numerical thresholds, zero-division hazards, empty collections, unicode variations, concurrent state mutations, and idempotency edge cases—are routinely neglected.

In critical components such as the Decision Policy Engine (`projects/decision-service`), PostgreSQL Row-Level Security evaluation (`projects/platform`), and Saga Compensating Actions (`projects/platform/internal/workflow`), unhandled edge cases lead to operational failures, unauthorized data leaks, or uncompensated external transactions.

## Decision
1. **Mandatory Property-Based Testing for Core Invariants**:
   - For all high-stakes deterministic business logic, unit test suites must complement traditional example-based tests with **Property-Based Testing (PBT)**.
   - **Go**: Use `pgregory.net/rapid` or `testing/quick` to generate pseudo-randomized inputs and assert invariant properties.
   - **Python**: Use `hypothesis` to explore domain inputs across string encodings, floating-point infinities/NaNs, and deeply nested dictionary structures.

2. **Core Domain Invariants to Assert via PBT**:
   - **Idempotency & Reversibility**: Re-applying an approved decision policy or compensating an ERP/FSM work order must yield the exact same state without producing duplicate side-effects.
   - **Calibrated Policy Monotonicity**: Higher calibrated safety risks or financial damage scores must never result in lower human-escalation requirements regardless of accompanying parameters.
   - **Tenant Context Partitioning**: Property tests fuzzing tenant tokens and IDs must prove that no parameter combination can return or modify data belonging to another tenant under PostgreSQL RLS simulation.

3. **CI Execution Budget**:
   - Standard PR runs execute PBT with a deterministic seed and bounded iteration limit (e.g., 100-200 iterations per property, completing within 5 seconds).
   - Nightly CI pipelines run extended shrinking and fuzzing iterations (10,000+ iterations per property) to uncover rare race conditions or numeric edge cases.

## Consequences

### Positive
- Surfaces obscure edge cases, overflow bugs, and invalid state transitions that LLM assistants fail to anticipate during code generation.
- Shrinking algorithms automatically reduce failing test cases to the absolute minimal reproducible input, accelerating debugging.
- Prevents agents from fabricating green test suites that only pass on hardcoded sample data.

### Negative / Trade-offs
- PBT tests require higher upfront intellectual investment to formulate invariant equations rather than static assertion values.
- Flaky tests can emerge if non-deterministic external dependencies (e.g., clock time, unseeded randomness) leak into property assertions.
