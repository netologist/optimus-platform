# Declarative Agent Skills over Static Python Specialist Classes

## Status
Accepted

## Context
Adding new enterprise domains (e.g., Asset Investment Planning, Fleet Scheduling) previously required authoring and deploying new Python specialist classes in `ai-runtime`. As multi-tenant deployments scale, tenants demand custom prompts, distinct tool permissions, and domain-specific evidence extraction without restarting or redeploying backend services.

## Decision
We decouple agent capabilities from runtime code by introducing declarative **Agent Skill Manifests** (`contracts/skills/*.yaml`). An Agent Skill defines the domain system prompt, the bound MCP tools, and the required JSON output schema. The AI Runtime loads skills dynamically, validating and exposing them based on tenant configuration.

## Consequences
- Enterprise integrations can introduce new agent capabilities simply by shipping a manifest and exposing MCP endpoints.
- Avoids code bloat and redeployment cycles in `ai-runtime`.
- Prompts and tool bindings become versioned, auditable, and testable assets outside compiled application code.
