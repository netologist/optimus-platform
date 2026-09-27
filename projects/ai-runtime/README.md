# ai-runtime

Python-based **multi-agent AI research engine**. Triggered by a single `InvestigateFailure` call from a Temporal activity; it accesses EAM/PLM/ERP enterprise systems via MCP tools, queries the knowledge base using hybrid RAG (pgvector + FTS + RRF), and returns a structured `EvidenceContext` using the Pydantic AI agent framework.

> **Boundary:** This service **does not make decisions and does not initiate any actions.** It only collects and recommends evidence. Decisions belong to `decision-service`, actions belong to the Temporal workflow.

## Architecture overview

```
Temporal Platform Worker
        │  POST /investigate
        ▼
┌───────────────────────────────────────────────┐
│  FastAPI  (port 8000)                         │
│  main.py  →  /investigate  →  PlannerAgent    │
│                                               │
│  ┌────────────────────────────────────────┐   │
│  │  InvestigationOrchestrator             │   │
│  │  (5-phase pipeline, LLM-free)          │   │
│  │                                        │   │
│  │  1. EAMSpecialist                      │   │
│  │     └─ eam.get_maintenance_history     │   │
│  │  2. PLMSpecialist                      │   │
│  │     └─ plm.search_documents            │   │
│  │  3. HybridRetriever (pgvector+FTS+RRF) │   │
│  │  4. ERPSpecialist                      │   │
│  │     └─ erp.get_inventory               │   │
│  │  5. Reranker + context synthesis       │   │
│  └────────────────────────────────────────┘   │
│                                               │
│  PlannerAgent (Pydantic AI)                   │
│  └─ LLM → EvidenceContext + recommendation    │
│                                               │
│  ProgressReporter → NATS (ephemeral, opt.)    │
└───────────────────────────────────────────────┘
```

## Package structure

```
projects/ai-runtime/
├── src/optimus_ai/
│   ├── main.py              # FastAPI application, /investigate route
│   ├── telemetry.py         # OTel bootstrap (OTLP gRPC → Jaeger)
│   ├── agents/
│   │   ├── planner.py       # PlannerAgent, EvidenceContext, PlannerDeps
│   │   └── specialists.py   # EAMSpecialist, PLMSpecialist, ERPSpecialist
│   ├── orchestration/
│   │   ├── orchestrator.py  # InvestigationOrchestrator (5 phases)
│   │   └── progress.py      # ProgressReporter (NATS / callback / log)
│   ├── llm/
│   │   └── provider.py      # LLM abstraction layer (4 providers)
│   ├── mcp/
│   │   └── client.py        # MCPClient — JSON-RPC 2.0 over HTTP
│   ├── rag/
│   │   ├── retriever.py     # HybridRetriever (pgvector + FTS + RRF)
│   │   ├── embeddings.py    # compute_embedding() — 384-dimensional
│   │   ├── reranker.py      # Reranker + DocumentMatch
│   │   └── ingest.py        # Document chunking + index CLI
│   ├── tools/
│   │   ├── eam.py           # EAMTools wrappers
│   │   ├── plm.py           # PLMTools wrappers
│   │   └── erp.py           # ERPTools wrappers
│   └── evaluation/
│       ├── dataset.py       # GOLDEN_DATASET — 10 golden scenarios
│       └── harness.py       # EvaluationHarness, EvaluationReport
├── tests/                   # pytest, pytest-asyncio
├── pyproject.toml           # uv, hatchling, Python ≥3.12
├── uv.lock                  # Pinned dependencies
└── Dockerfile               # python:3.12-slim → uv sync → uvicorn
```

## HTTP API

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check |
| `POST` | `/investigate` | Start a new investigation |
| `GET` | `/metrics` | Prometheus metrics |

### Investigation request

```bash
curl -X POST http://localhost:8000/investigate \
  -H "Content-Type: application/json" \
  -d '{
    "asset_id": "P-104",
    "tenant_id": "acme",
    "symptom": "repeated overheating"
  }'
```

### Response — `EvidenceContext`

```json
{
  "asset_id": "P-104",
  "tenant_id": "acme",
  "findings": {
    "maintenance_history": { "failures_last_30_days": 4, "events": [] },
    "plm_findings": "PLM-COOL-4021 §4.2: Known cooling-system failure mode",
    "spare_part_id": "SP-COOL-9981",
    "spare_part_available": true,
    "inventory_level": 6
  },
  "recommendation": "Immediate P1 maintenance required. Cooling thermostat replacement recommended.",
  "confidence": 0.92
}
```

## 5-Phase investigation pipeline

```
Phase 1 — EAM Lookup
  EAMSpecialist → MCP: eam.get_maintenance_history(asset_id, tenant_id)
  → failures in last 30 days, maintenance history

Phase 2 — PLM Search
  PLMSpecialist → MCP: plm.search_documents(query)
  → maintenance manual findings, known failure modes

Phase 3 — Hybrid RAG (activates when PLM findings are absent or spare part is uncertain)
  HybridRetriever:
    compute_embedding(query) → 384-dim normalized vector
    PostgreSQL CTE:
      FTS  → plainto_tsquery / ts_rank, top-20
      HNSW → embedding <=> $vec (pgvector), top-20
    RRF   → score = 1/(60+fts_rank) + 1/(60+vec_rank)
    Multi-tenant RLS filter
    Reranker → DocumentMatch list

Phase 4 — ERP Inventory
  ERPSpecialist → MCP: erp.get_inventory(part_id, tenant_id)
  → stock information, warehouse, lead time in days

Phase 5 — Synthesis
  PlannerAgent (Pydantic AI + LLM)
  → EvidenceContext + recommendation text
```

## LLM Provider abstraction

Selected via the `LLM_PROVIDER` environment variable:

| Provider | Value | Default model | Usage |
|---|---|---|---|
| **mock** | `mock` | — | CI/CD, tests — zero cost |
| **ollama** | `ollama` | `qwen2.5:0.5b` | Local development — free |
| **anthropic** | `anthropic` | `claude-3-5-sonnet-latest` | Demo mode — highest quality |
| **openai** | `openai` | `gpt-4o` | Alternative hosted option |

```python
# Abstraction in provider.py
def get_model(settings: LLMSettings) -> Model:
    match settings.llm_provider:
        case "ollama":    return OpenAIChatModel(model, base_url=ollama_url)
        case "anthropic": return AnthropicModel(model)
        case "openai":    return OpenAIChatModel(model)
        case "mock":      return TestModel(...)
```

This abstraction ensures CI always runs with the `mock` provider (zero cost, deterministic); for demo presentations it can be switched to `anthropic` via an environment variable.

## RAG pipeline — Hybrid Search

```sql
-- PostgreSQL CTE executed by HybridRetriever (simplified)
WITH
  fts AS (
    SELECT chunk_id, ts_rank(tsv, query) AS score, row_number() OVER (ORDER BY score DESC) AS rank
    FROM document_chunks, plainto_tsquery('english', $query) query
    WHERE tsv @@ query AND tenant_id = current_setting('app.current_tenant')
    LIMIT 20
  ),
  vec AS (
    SELECT chunk_id, embedding <=> $embedding AS dist, row_number() OVER (ORDER BY dist) AS rank
    FROM document_chunks
    WHERE tenant_id = current_setting('app.current_tenant')
    LIMIT 20
  )
SELECT chunk_id,
       1.0/(60 + COALESCE(fts.rank, 1e6)) + 1.0/(60 + COALESCE(vec.rank, 1e6)) AS rrf_score
FROM fts FULL OUTER JOIN vec USING (chunk_id)
ORDER BY rrf_score DESC;
```

The embedding model uses a deterministic hash projection (384 dimensions, no API call) — embedding API cost is zero in CI.

## Evaluation system

The `evaluation/` package detects agent regressions:

- **10 golden scenarios** (`GOLDEN_DATASET`): expected MCP tool calls and their arguments are defined for each scenario.
- **MockHarnessMCPClient**: records tool calls instead of calling real MCP.
- **EvaluationHarness**: runs the agent for each scenario and compares structurally (looks at tool calls, not LLM output text).
- Runs as **advisory** (non-blocking) in CI initially; switched to blocking once the golden set stabilises.

```bash
# Run the evaluation
uv run python -m optimus_ai.evaluation.harness
```

## Dependencies

| Package | Min Version | Purpose |
|---|---|---|
| `pydantic-ai` | 0.0.18 | Agent framework |
| `fastapi` | 0.115.0 | HTTP API |
| `uvicorn` | 0.34.0 | ASGI server |
| `anthropic` | 0.40.0 | Claude API |
| `openai` | 1.50.0 | OpenAI + Ollama-compatible endpoint |
| `asyncpg` | 0.30.0 | PostgreSQL async driver |
| `pgvector` | 0.3.6 | pgvector asyncpg binding |
| `httpx` | 0.28.0 | MCP HTTP calls |
| `numpy` | 2.0.0 | Embedding vector operations |
| `opentelemetry-sdk` | 1.27.0 | OTel tracing |

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `LLM_PROVIDER` | `mock` | `mock` / `ollama` / `anthropic` / `openai` |
| `ANTHROPIC_API_KEY` | — | For Anthropic demo mode |
| `ANTHROPIC_MODEL` | `claude-3-5-sonnet-latest` | Model selection |
| `OLLAMA_BASE_URL` | `http://127.0.0.1:11434` | Local Ollama endpoint |
| `OLLAMA_MODEL` | `qwen2.5:0.5b` | Ollama model name |
| `MCP_SERVER_URL` | — | integration-mocks MCP endpoint |
| `DATABASE_URL` | — | PostgreSQL connection string (for RAG) |
| `NATS_URL` | — | For live progress streaming (optional) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `jaeger.optimus.svc:4317` | Jaeger gRPC target |

## Local development

```bash
# Install dependencies
uv sync

# Linting
uv run ruff check .
uv run mypy .

# Tests
uv run pytest

# Start server (with mock LLM)
LLM_PROVIDER=mock uv run uvicorn optimus_ai.main:app --reload --port 8000

# Document indexing
uv run python -m optimus_ai.rag.ingest \
  --tenant acme --source /path/to/docs --chunk-size 512
```

## Build

```bash
docker build -t localhost:5001/optimus/ai-runtime:dev .
docker push localhost:5001/optimus/ai-runtime:dev
```

## Related components

| Component | Relationship |
|---|---|
| [`platform`](../platform/README.md) | The `InvestigateFailure` activity calls this service |
| [`integration-mocks`](../integration-mocks/README.md) | MCPClient calls EAM/PLM/ERP tools from here |
| [`decision-service`](../decision-service/README.md) | This service does not make decisions — decisions flow via the platform's Temporal workflow to decision-service |
