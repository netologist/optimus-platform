# Compiled Temporal Workflows with Declarative CRD/Database Routing

## Status
Accepted

## Context
Multi-tenant enterprise platforms require different workflows for different tenants (e.g., auto-dispatch vs. human approval, maintenance vs. capital replacement). Implementing business workflows as Kubernetes CRDs violates the level-triggered model of Kubernetes when long-running sleeps, human approval waits, and distributed Saga rollbacks are required. Conversely, hardcoding a single Go workflow limits reusability across tenants.

## Decision
Workflow execution logic remains compiled, type-safe Go code within Temporal Workers, leveraging Temporal's native durable execution, 24h timers, and Saga compensation. Tenant-specific workflow selection and parameter thresholds (`autoApproveThreshold`, `escalateAfterHours`) are declared in Kubernetes CRDs (`EnterpriseEnvironment`) and synchronized to a PostgreSQL routing table (`tenant_workflow_routes`). When an operational signal arrives, the platform dynamically resolves and triggers the configured Temporal workflow.

## Consequences
- Preserves Kubernetes reconciliation purity: CRDs declare desired configuration, not step-by-step transaction state.
- Preserves Temporal's durability and testability: Workflows are versioned Go code tested with `temporal.TestSuite`.
- Tenants can change their operating procedures and risk thresholds dynamically via CRDs without redeploying workflow worker binaries.
