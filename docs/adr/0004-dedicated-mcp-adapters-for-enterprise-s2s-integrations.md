# Dedicated Containerized MCP Adapters for Enterprise System-to-System Integrations

## Status
Accepted

## Context
Connecting disparate enterprise platforms across Operational Intelligence (OI), Product Lifecycle Management (PLM), Enterprise Asset Management (EAM), and Field Service Management (FSM) requires interfacing with heterogeneous legacy protocols: OData v4, SOAP/XML, OPC-UA, MQTT, and SAP RFCs. Embedding protocol-specific drivers into the core Platform API creates monolithic coupling (reminiscent of legacy Enterprise Service Buses like Camel or MuleSoft), while performing protocol transformation inside Kong Gateway leaks business logic into the edge layer.

## Decision
We enforce an architectural boundary where core platform services (Platform API, Temporal Workers, AI Runtime) communicate with external enterprise systems strictly via the Model Context Protocol (MCP JSON-RPC 2.0).
1. For each enterprise system type (e.g., IFS Cloud EAM, Siemens Teamcenter PLM, SAP S/4HANA ERP), a dedicated, lightweight **Enterprise MCP Adapter** container runs as an independent deployment or sidecar.
2. The adapter translates vendor-specific protocols (OData, SOAP, RFC) into standardized MCP tool calls (`tools/call`, `tools/list`).
3. The Kubernetes Operator provisions and manages these adapters via the `EnterpriseIntegration` CRD, handling credentials, rate limits, and endpoint discovery.

## Consequences
- Core platform and AI agents are 100% decoupled from third-party vendor protocol churn.
- Adding a new enterprise system only requires authoring a single isolated MCP adapter, without touching platform binaries or re-evaluating core schemas.
- Avoids the maintenance overhead and fragility of monolithic ESB architectures.
- Enables canary deployments, distinct security perimeters, and isolated scaling for each enterprise integration adapter.
