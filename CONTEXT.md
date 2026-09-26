# Optimus Platform — Ubiquitous Language & Domain Model

The unified domain model for `optimus`, an AI-native enterprise integration and operational orchestration platform.

## Language

### Ingestion & Observability

**Operational Signal**:
An immutable telemetry anomaly, event, or incident record emitted by an enterprise system (OI/SCADA/IoT) that initiates an operational investigation.
_Avoid_: Alert, alarm, notification, ticket

**Tenant**:
An isolated enterprise organization with strict data partitioning, distinct integration endpoints, custom policies, and dedicated security boundaries.
_Avoid_: Customer, account, workspace, client

---

### Agentic AI & Knowledge Retrieval

**Evidence Context**:
A structured, verifiable synthesis of data collected by AI specialist agents across enterprise systems (EAM, PLM, ERP, FSM) via Model Context Protocol (MCP) and Hybrid RAG.
_Avoid_: LLM output, agent thoughts, prompt response, summary
**Agent Skill Manifest**:
A declarative YAML/JSON document specifying an agent persona, bound MCP tools, and a structured output contract, allowing dynamic capability loading without code changes.
_Avoid_: Prompt file, tool config, plugin spec


**Agent Skill**:
A high-level agent capability comprising a specialized system prompt, a bound set of MCP tools, and deterministic output validation criteria.
_Avoid_: Tool, function call, prompt snippet

**Enterprise Knowledge Source**:
A declarative Kubernetes resource defining a tenant documentation repository (S3, SharePoint, SMB), embedding parameters, and sync schedule, reconciled via background ingestion CronJobs.
_Avoid_: Document bucket, RAG folder, file drop

**Federated Knowledge Gateway**:
A hybrid retrieval mechanism that merges tenant-partitioned pgvector semantic search with external vendor search APIs delegated via Model Context Protocol (MCP).
_Avoid_: Universal search, meta search, doc aggregator

---

### Decision Intelligence & Governance

**Governed Decision**:
A typed, probabilistic inference emitted by a decision model (SystemOne wire protocol) and validated against deterministic policy rules, producing calibrated risk scores and human-in-the-loop flags.
_Avoid_: LLM decision, prediction, suggestion, judgment

**Decision Policy**:
A versioned, deterministic rule set that evaluates a model's calibrated probabilities to enforce enterprise guardrails (e.g., mandating human supervisor sign-off on high safety risks).
_Avoid_: Business logic, hardcoded if-statement, prompt guardrail

---

### Orchestration & Enterprise Action

**Durable Business Workflow**:
A fault-tolerant, stateful execution unit orchestrated by Temporal that coordinates multi-system activities, signal-based approvals, and long-running timers across days or weeks.
_Avoid_: Pipeline, batch job, background worker, script

**Tenant Workflow Route**:
A declarative mapping binding incoming operational signal patterns to pre-compiled Temporal workflows with tenant-specific execution parameters.
_Avoid_: Event rule, workflow trigger, routing rule

**Enterprise Integration**:
A managed connection to an external enterprise application (EAM, PLM, ERP, FSM, AIP) providing authentication, rate limiting, and an MCP tool interface.
_Avoid_: Plugin, connector, webhook, adapter

**Compensating Action**:
An idempotent operational transaction executed by a Temporal workflow to undo previously committed external side-effects when a downstream step in a Saga fails.
_Avoid_: Rollback, undo, clean-up script
