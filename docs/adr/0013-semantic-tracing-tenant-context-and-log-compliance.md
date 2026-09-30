# Mandatory Semantic Tracing, Tenant Context Propagation, and Log Envelope Compliance

## Status
Accepted

## Context
In a multi-tenant enterprise orchestration platform where requests span HTTP APIs, asynchronous Redpanda queues, Temporal durable workflows, Python agent runtimes, and local Ollaya decision stubs, troubleshooting failures depends entirely on distributed telemetry. 

AI coding assistants frequently break distributed traces by:
1. Omitting `context.Context` or dropping W3C `traceparent` headers when initiating outgoing HTTP or Kafka calls.
2. Forgetting to pass `tenant_id` or `TenantContext` in repository functions, activities, or background threads.
3. Writing unformatted prose logs with arbitrary field keys (e.g., using `tenantId`, `tenant`, or `tenant_uuid` interchangeably), rendering centralized Elasticsearch/Kibana indexing and filtering useless.

Telemetry discipline cannot be left to developer or agent discretion; it must be enforced by framework primitives and validated by test assertions.

## Decision
1. **Framework-Level Context & Trace Propagation**:
   - Outgoing HTTP clients and messaging producers across all Go and Python services must automatically inject W3C `traceparent` and `tracestate` headers using OpenTelemetry propagators.
   - Incoming transport middleware must reject any operational request lacking a valid `X-Tenant-ID` header or authenticated tenant token.
   - Temporal activity and workflow inputs must strictly embed `tenant.TenantContext` as their first parameter.

2. **Elastic Common Schema (ECS) Log Envelope Compliance**:
   - All log emissions must follow structured JSON logging complying with Elastic Common Schema (ECS).
   - The logging envelope must deterministically inject:
     - `trace.id` and `span.id` (extracted from the active OTel context).
     - `tenant.id` (extracted from `TenantContext`).
     - `service.name` and `service.version`.
   - Ad-hoc string formatting (`fmt.Printf`, `print()`) is banned via static analysis linters (`forbidigo` in Go, `ruff` rules in Python).

3. **Mechanical Verification in CI and E2E**:
   - Unit and integration tests must assert that span context is actively propagated through the transport and activity boundaries.
   - The Ginkgo E2E test suite asserts that spans recorded in Jaeger for scenario runs (`IngestSignal` -> `InvestigateFailure` -> `RunDecision` -> `CreateWorkOrder`) share an unbroken trace ID across all participating containers.
   - Centralized logging tests verify that messages ingested into Elasticsearch `optimus-logs` contain the indexed `tenant.id` and `trace.id` attributes.

## Consequences

### Positive
- Guarantees seamless end-to-end traceability from the initial operational signal to the final FSM work order in Jaeger and Kibana.
- Guarantees strict multi-tenant auditability: zero unpartitioned or unidentifiable operations in production logs.
- Eliminates manual log-tracing guesswork for human operators and diagnostic agents.

### Negative / Trade-offs
- Requires passing `context.Context` rigorously across all Go API boundaries; no context dropping permitted.
- Structured logging envelopes slightly increase byte payload volume in stdout log streams compared to bare text strings.
