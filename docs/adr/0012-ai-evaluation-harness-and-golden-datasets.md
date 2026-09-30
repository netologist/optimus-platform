# AI Evaluation Harness and Golden Dataset Regression Prevention

## Status
Accepted

## Context
When modifying agent prompts, updating LLM model versions (e.g., local Ollama models vs. hosted Anthropic/OpenAI providers), or refactoring orchestration logic in `projects/ai-runtime`, prompt drift and reasoning regressions occur silently. Unlike traditional software where regressions trigger compiler errors or unit test exceptions, an LLM regression manifests as subtle degraded behavior: selecting suboptimal MCP tools, omitting critical evidence fields, misinterpreting maintenance manuals, or producing hallucinated parameters.

Asserting against raw generated prose using string containment (e.g., `assert "overheating" in response`) produces brittle, non-deterministic tests that break on minor wording changes while failing to verify actual reasoning fidelity.

## Decision
1. **Structural Golden Dataset Harness**:
   - Maintain a curated repository of ground-truth scenarios under `projects/ai-runtime/src/optimus_ai/evaluation/golden_dataset/`.
   - Each golden scenario specifies:
     - Input operational signal and tenant context.
     - Expected sequence of MCP tool calls (`tool_name` and JSON Schema validation of parameters).
     - Expected evidence synthesis keys (`EvidenceContext`).
     - Expected questions formulated for the Ollaya/SystemOne decision engine.

2. **Deterministic Evaluation Assertion Strategy**:
   - The test harness mocks LLM external calls using recorded golden transcripts during fast CI runs, verifying that the orchestration state machine traverses the correct tool DAG.
   - For live evaluation runs (`mise run eval` or nightly CI):
     - Agents run against a deterministic local model (e.g., Ollama `llama3.2` or deterministic seed).
     - Evaluation does **not** assert on string equality of prose; it asserts strictly on **structural correctness**:
       1. **Tool Precision & Recall**: Did the planner invoke `plm.search_documents` and `eam.get_asset`? Were extraneous or hallucinated tools avoided?
       2. **Parameter Validity**: Were the arguments passed to MCP tools compliant with `contracts/mcp/*.json`?
       3. **Calibrated Decision Formulation**: Did the synthesized evidence map accurately into the required SystemOne questions?

3. **Regression Gating**:
   - Compute aggregate benchmark scores: **Tool Selection Accuracy**, **Schema Conformance Rate**, and **Decision Alignment Score**.
   - A pull request that drops the composite benchmark score below the established baseline (target: 95%) is rejected by CI as an agentic regression.

## Consequences

### Positive
- Replaces brittle substring matching with verifiable, structural evaluation of agent behavior.
- Enables safe prompt engineering and model upgrades without fear of silent operational failures.
- Provides a quantitative quality score demonstrating platform robustness.

### Negative / Trade-offs
- Curating and maintaining representative golden datasets requires continuous domain expertise when new operational scenarios are introduced.
- Live model evaluation runs require compute resources and may exhibit minor variance if external stochastic models are evaluated without strict temperature=0 constraints.
