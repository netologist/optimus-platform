from __future__ import annotations

import numpy as np
import pytest

from optimus_ai.rag.embeddings import compute_embedding, compute_embeddings_batch
from optimus_ai.rag.reranker import DocumentMatch, Reranker
from optimus_ai.rag.retriever import HybridRetriever


def test_embedding_dimensions_and_normalization():
    text = "Recurrent overheating in pump cooling manifolds (SP-COOL-9981)"
    vec = compute_embedding(text, dim=384)

    assert len(vec) == 384, "Embedding vector must have exactly 384 dimensions"
    norm = np.linalg.norm(vec)
    assert np.isclose(norm, 1.0, atol=1e-4), "Embedding vector must be L2 normalized"


def test_embedding_semantic_separation():
    v_pump = np.array(compute_embedding("Pump cooling system failure thermostat overheating"))
    v_pump_rel = np.array(compute_embedding("Cooling loop thermostat replacement for P-104"))
    v_unrel = np.array(compute_embedding("Human resources quarterly payroll taxation guidelines"))

    sim_related = np.dot(v_pump, v_pump_rel)
    sim_unrelated = np.dot(v_pump, v_unrel)

    assert sim_related > sim_unrelated, "Relevant engineering queries must yield higher similarity"


def test_batch_embeddings():
    texts = ["doc one", "doc two", "doc three"]
    vectors = compute_embeddings_batch(texts, dim=384)
    assert len(vectors) == 3
    for v in vectors:
        assert len(v) == 384


def test_reranker_scoring_and_bonuses():
    reranker = Reranker()
    query = "P-104 overheating thermostat failure SP-COOL-9981"

    candidates = [
        DocumentMatch(
            document_id="PLM-GEN-100",
            title="General Plant Safety Overview",
            section="§1.0",
            content="General electrical safety precautions in plant facilities.",
            score=0.4,
        ),
        DocumentMatch(
            document_id="PLM-COOL-4021",
            title="Centrifugal Pump Cooling Loop Maintenance Manual",
            section="§4.2",
            content="Recurrent overheating in pump cooling manifolds typically indicates thermostat bypass failure (part SP-COOL-9981).",
            score=0.6,
            spare_part="SP-COOL-9981",
        ),
    ]

    reranked = reranker.rerank(query, candidates, top_k=2)

    assert len(reranked) == 2
    # The technical manual matching P-104 and SP-COOL-9981 must rank first
    assert reranked[0].document_id == "PLM-COOL-4021"
    assert reranked[0].score > reranked[1].score
    assert reranked[0].spare_part == "SP-COOL-9981"


@pytest.mark.asyncio
async def test_hybrid_retriever_search_and_ingestion():
    retriever = HybridRetriever()

    # Test initial retrieval
    matches = await retriever.search("P-104 overheating cooling", tenant_id="acme", limit=2)
    assert len(matches) > 0
    assert any("PLM-COOL-4021" in m.document_id for m in matches)
    assert any(m.spare_part == "SP-COOL-9981" for m in matches)

    # Test dynamic ingestion
    await retriever.ingest_document(
        tenant_id="acme",
        document_id="PLM-CUSTOM-99",
        title="Custom Auxiliary Fan Guide",
        content="Auxiliary cooling fan blade fatigue causes severe vibration. Replace with part SP-FAN-7711 (§3.1).",
    )

    custom_matches = await retriever.search("auxiliary cooling fan vibration SP-FAN-7711", tenant_id="acme", limit=1)
    assert len(custom_matches) > 0
    assert custom_matches[0].document_id == "PLM-CUSTOM-99"
    assert custom_matches[0].spare_part == "SP-FAN-7711"
