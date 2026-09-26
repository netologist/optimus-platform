# Dual Messaging Architecture: Redpanda for Durable Events and NATS Core for Ephemeral Agent Telemetry

## Status
Accepted

## Context
An AI-native enterprise platform produces two fundamentally distinct categories of events:
1. **Durable Business Events:** S2S audit records, work order dispatches, and inventory state transitions that require transactional guarantees, long-term retention, consumer group replayability, and strict ordering.
2. **Ephemeral Agent Telemetry:** High-frequency, sub-second status signals ("planner parsing document", "specialist executing MCP call", "decision engine computing forward pass") published to inform human operators in real-time UI dashboards.

Routing ephemeral agent progress through Redpanda/Kafka introduces unnecessary disk I/O, partition retention management, and consumer offset churn for data that is worthless if not observed immediately. Conversely, relying on in-memory server channels (WebSockets in Platform API) breaks horizontal scalability across multiple application pod replicas.

## Decision
We adopt a **Dual Messaging Architecture**:
1. **Redpanda (Kafka Wire Protocol):** Serves as the sole authoritative, durable domain event log. Populated strictly via the Transactional Outbox pattern with W3C `traceparent` propagation for business transactions and compliance audit trails.
2. **NATS Core (No JetStream required):** Serves as the stateless, ephemeral pub/sub fabric for real-time agent execution telemetry. The AI Runtime publishes transient progress packets to `agent.progress.{tenant}.{workflow_id}`. Real-time frontends or CLI observers subscribe directly with sub-millisecond fan-out and zero disk persistence.
3. System correctness never depends on NATS message delivery: dropping a progress packet only degrades live UI feedback, never core enterprise state.

## Consequences
- Protects Redpanda from high-volume, transient telemetry noise.
- Provides true sub-millisecond, multi-pod WebSocket/SSE streaming without complex state synchronization between backend replicas.
- Clarifies architectural boundaries: Redpanda is for truth and audit; NATS is for live observation.
