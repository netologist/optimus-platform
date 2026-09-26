# Declarative Versioned Decision Policy Engine

## Status
Accepted

## Context
Enterprise operational environments operate under vastly differing safety regulations and risk tolerances. Regulated facilities (nuclear, aerospace, defense) mandate human sign-off on even minor thermal anomalies or modest confidence drops, while high-throughput logistics hubs favor autonomous dispatch for sub-$1,000 work orders. Hardcoding policy rules into the Go binary (`policy_v1`) forces service re-deployments across all tenants whenever a risk threshold is tuned. Conversely, introducing a full Open Policy Agent (OPA) / Rego stack introduces cognitive overhead for operations teams who primarily need to adjust numeric confidence thresholds and risk enumerations.

## Decision
We implement a **Declarative Versioned Decision Policy Engine**:
1. Policies are defined as structured JSON/YAML specifications (stored in `contracts/policies/` and persisted in the `decision_policies` table).
2. The Go `decision-service` evaluates deterministic threshold rules (`safety_risk_in`, `min_confidence`, `max_auto_dispatch_cost_usd`) against the calibrated probabilities returned by the SystemOne/Ollaya model.
3. Every evaluated decision records the exact `policy_id` and `policy_version` into the `decision_audit` table alongside the model output for immutable compliance auditing.
4. **Note on OPA/Rego:** Open Policy Agent (OPA) with Rego is recognized as a valid extension point for complex multi-attribute enterprise authorization rules. If a tenant requires complex external data lookups or nested relational policy graphs in the future, the policy engine interface allows delegating evaluation to an in-process Rego evaluator (`open-policy-agent/opa/rego`) without changing the upstream SystemOne or Temporal interfaces.

## Consequences
- Risk thresholds, approval gates, and escalation conditions can be versioned, tested, and updated per tenant without redeploying code.
- Fully auditable: Regulators can verify exactly which policy rules and version governed an automated dispatch.
- Keeps policy evaluation sub-millisecond in Go without requiring operations teams to learn Rego syntax for standard threshold configurations.
