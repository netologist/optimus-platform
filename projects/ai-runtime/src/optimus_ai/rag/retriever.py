from __future__ import annotations

import logging
import os
import re
from typing import Any

from .embeddings import compute_embedding
from .reranker import DocumentMatch, Reranker

logger = logging.getLogger(__name__)

# Pattern for detecting spare parts in engineering manuals (e.g. SP-COOL-9981, SP-TURB-5520)
SPARE_PART_PATTERN = re.compile(r"\b(SP-[A-Z]+-[0-9]+)\b")
SECTION_PATTERN = re.compile(r"(§\s*\d+(\.\d+)?|Section\s+\d+(\.\d+)?)", re.IGNORECASE)


class HybridRetriever:
    """Enterprise RAG Retriever implementing PostgreSQL 17 + pgvector + FTS Hybrid Search (ADR-009).

    Queries PostgreSQL document_chunks using:
    1. Full-Text Search (tsvector / ts_rank)
    2. Vector Cosine Distance (pgvector HNSW index)
    3. Reciprocal Rank Fusion (RRF) to merge keyword and semantic signals
    4. Cross-Encoder reranking via Reranker

    Falls back to a deterministic in-memory store when DATABASE_URL is unset.
    """

    def __init__(
        self,
        database_url: str | None = None,
        reranker: Reranker | None = None,
    ):
        self.database_url = (
            database_url
            or os.getenv("DATABASE_URL")
            or os.getenv("PG_DATABASE_URL")
        )
        self.reranker = reranker or Reranker()
        self._pool: Any = None
        self._fallback_store: list[dict[str, Any]] = self._init_default_fallback_documents()

    def _init_default_fallback_documents(self) -> list[dict[str, Any]]:
        """Pre-loaded technical PLM documentation for zero-dependency offline mode."""
        return [
            {
                "document_id": "PLM-COOL-4021",
                "title": "Centrifugal Pump Cooling Loop Maintenance Manual",
                "section": "§4.2",
                "content": (
                    "Recurrent overheating in pump cooling manifolds typically indicates thermostat bypass failure "
                    "(part SP-COOL-9981). Replace thermostat and flush secondary cooling line immediately to prevent bearing seizure."
                ),
                "spare_part": "SP-COOL-9981",
                "tenant_id": "acme",
            },
            {
                "document_id": "PLM-TURB-9011",
                "title": "Industrial Gas Turbine Radial Bearing Service Bulletin",
                "section": "§8.1",
                "content": (
                    "Journal bearing sleeve micro-cracking and hydrodynamic oil wedge collapse cause severe vibration. "
                    "Requires emergency journal sleeve replacement (part SP-TURB-5520)."
                ),
                "spare_part": "SP-TURB-5520",
                "tenant_id": "acme",
            },
            {
                "document_id": "PLM-VALV-3310",
                "title": "Severe Service Control Valve Stem & Seal Overhaul Procedure",
                "section": "§3.4",
                "content": (
                    "High pressure hydraulic fluid leakage at actuator stem indicates stem packing extrusion. "
                    "Install high-temperature fluoroelastomer seal kit (part SP-VALV-1200)."
                ),
                "spare_part": "SP-VALV-1200",
                "tenant_id": "acme",
            },
            {
                "document_id": "PLM-MOT-2044",
                "title": "AC Induction Motor Thermal Protection & Stator Troubleshooting",
                "section": "§2.1",
                "content": (
                    "Stator overcurrent and phase imbalance indicate winding insulation breakdown. "
                    "Replace stator core assembly (part SP-MOT-8812)."
                ),
                "spare_part": "SP-MOT-8812",
                "tenant_id": "acme",
            },
            {
                "document_id": "PLM-COMP-6112",
                "title": "Rotary Screw Compressor Air End Maintenance Guide",
                "section": "§5.3",
                "content": (
                    "Discharge differential pressure drop occurs when internal rotor sealing clearance degrades. "
                    "Service rotary screw rotor set (part SP-COMP-7740)."
                ),
                "spare_part": "SP-COMP-7740",
                "tenant_id": "acme",
            },
        ]

    async def _get_pool(self) -> Any:
        if not self.database_url:
            return None

        if self._pool is None:
            try:
                import asyncpg  # type: ignore[import-untyped]
                from pgvector.asyncpg import register_vector

                clean_url = self.database_url.replace("postgresql+asyncpg://", "postgres://")
                self._pool = await asyncpg.create_pool(clean_url, min_size=1, max_size=5)

                # Register pgvector type on new connections
                async with self._pool.acquire() as conn:
                    await register_vector(conn)
                logger.info("HybridRetriever connected to PostgreSQL with pgvector extension enabled.")
            except Exception as e:
                logger.warning(
                    "Failed to initialize PostgreSQL pool (%s). Operating in in-memory fallback mode.", e
                )
                self._pool = None

        return self._pool

    async def search(
        self,
        query: str,
        tenant_id: str = "acme",
        limit: int = 3,
    ) -> list[DocumentMatch]:
        """Perform Hybrid RRF (pgvector + FTS) search against PostgreSQL with RLS and Reranker."""
        pool = await self._get_pool()

        if pool is not None:
            try:
                candidates = await self._search_postgres_hybrid(pool, query, tenant_id, limit=limit * 3)
                if candidates:
                    return self.reranker.rerank(query, candidates, top_k=limit)
            except Exception as e:
                logger.warning("PostgreSQL hybrid search error: %s. Falling back to in-memory index.", e)

        # Fallback in-memory search
        candidates = self._search_fallback(query, tenant_id)
        return self.reranker.rerank(query, candidates, top_k=limit)

    async def _search_postgres_hybrid(
        self,
        pool: Any,
        query: str,
        tenant_id: str,
        limit: int = 10,
    ) -> list[DocumentMatch]:
        """Execute Reciprocal Rank Fusion between Full-Text Search and pgvector HNSW."""
        import numpy as np

        query_vector = np.array(compute_embedding(query), dtype=np.float32)

        sql = """
        WITH fts_results AS (
            SELECT dc.id, dc.document_id, dc.content, COALESCE(d.title, 'PLM Technical Document') AS title,
                   ROW_NUMBER() OVER (ORDER BY ts_rank(dc.tsv, plainto_tsquery('english', $1)) DESC) AS fts_rank
            FROM document_chunks dc
            LEFT JOIN documents d ON d.id = dc.document_id AND d.tenant_id = dc.tenant_id
            WHERE dc.tenant_id = $2 AND dc.tsv @@ plainto_tsquery('english', $1)
            LIMIT 20
        ),
        vector_results AS (
            SELECT dc.id, dc.document_id, dc.content, COALESCE(d.title, 'PLM Technical Document') AS title,
                   ROW_NUMBER() OVER (ORDER BY dc.embedding <=> $3) AS vec_rank
            FROM document_chunks dc
            LEFT JOIN documents d ON d.id = dc.document_id AND d.tenant_id = dc.tenant_id
            WHERE dc.tenant_id = $2 AND dc.embedding IS NOT NULL
            LIMIT 20
        )
        SELECT COALESCE(f.id, v.id) AS chunk_id,
               COALESCE(f.document_id, v.document_id) AS document_id,
               COALESCE(f.title, v.title) AS title,
               COALESCE(f.content, v.content) AS content,
               (COALESCE(1.0 / (60 + f.fts_rank), 0.0) + COALESCE(1.0 / (60 + v.vec_rank), 0.0)) AS rrf_score,
               CASE
                   WHEN f.id IS NOT NULL AND v.id IS NOT NULL THEN 'hybrid'
                   WHEN f.id IS NOT NULL THEN 'fts'
                   ELSE 'vector'
               END AS rank_type
        FROM fts_results f
        FULL OUTER JOIN vector_results v ON f.id = v.id
        ORDER BY rrf_score DESC
        LIMIT $4;
        """

        candidates: list[DocumentMatch] = []
        async with pool.acquire() as conn:
            async with conn.transaction():
                # Enforce multi-tenant Row-Level Security
                await conn.execute("SET LOCAL app.current_tenant = $1", tenant_id)
                rows = await conn.fetch(sql, query, tenant_id, query_vector, limit)

                for r in rows:
                    content_str = r["content"]
                    part_match = SPARE_PART_PATTERN.search(content_str)
                    sec_match = SECTION_PATTERN.search(content_str)

                    candidates.append(
                        DocumentMatch(
                            document_id=r["document_id"],
                            title=r["title"],
                            section=sec_match.group(0) if sec_match else "§General",
                            content=content_str,
                            score=float(r["rrf_score"]),
                            spare_part=part_match.group(1) if part_match else None,
                            rank_type=r["rank_type"],
                        )
                    )

        return candidates

    def _search_fallback(self, query: str, tenant_id: str) -> list[DocumentMatch]:
        """In-memory hybrid scoring over embedded technical documents for offline environments."""
        q_lower = query.lower()
        tokens = set(re.findall(r"\b\w+\b", q_lower))
        matches: list[DocumentMatch] = []

        for doc in self._fallback_store:
            if doc.get("tenant_id") and doc["tenant_id"] != tenant_id:
                continue

            content = doc["content"]
            title = doc["title"]
            doc_text = (title + " " + content).lower()

            # Simple lexical overlap
            doc_tokens = set(re.findall(r"\b\w+\b", doc_text))
            overlap = tokens.intersection(doc_tokens)
            if not overlap and not any(t in doc_text for t in tokens if len(t) >= 4):
                continue

            score = 0.5 + 0.1 * len(overlap)
            matches.append(
                DocumentMatch(
                    document_id=doc["document_id"],
                    title=title,
                    section=doc["section"],
                    content=content,
                    score=min(0.95, score),
                    spare_part=doc.get("spare_part"),
                    rank_type="in-memory",
                )
            )

        # Default fallback match if no specific match found
        if not matches:
            default_doc = self._fallback_store[0]
            matches.append(
                DocumentMatch(
                    document_id=default_doc["document_id"],
                    title=default_doc["title"],
                    section=default_doc["section"],
                    content=default_doc["content"],
                    score=0.85,
                    spare_part=default_doc.get("spare_part"),
                    rank_type="in-memory",
                )
            )

        return matches

    async def ingest_document(
        self,
        tenant_id: str,
        document_id: str,
        title: str,
        content: str,
        doc_type: str = "MANUAL",
    ) -> None:
        """Ingest and chunk a document, computing vector embeddings and storing in PostgreSQL."""
        pool = await self._get_pool()
        if pool is not None:
            embedding = compute_embedding(content)
            async with pool.acquire() as conn:
                async with conn.transaction():
                    await conn.execute("SET LOCAL app.current_tenant = $1", tenant_id)
                    await conn.execute(
                        """
                        INSERT INTO documents (id, tenant_id, title, doc_type)
                        VALUES ($1, $2, $3, $4)
                        ON CONFLICT (tenant_id, id) DO UPDATE SET title = EXCLUDED.title;
                        """,
                        document_id,
                        tenant_id,
                        title,
                        doc_type,
                    )
                    await conn.execute(
                        """
                        INSERT INTO document_chunks (tenant_id, document_id, chunk_index, content, embedding)
                        VALUES ($1, $2, $3, $4, $5);
                        """,
                        tenant_id,
                        document_id,
                        0,
                        content,
                        embedding,
                    )
            return

        # In-memory store ingestion
        part_match = SPARE_PART_PATTERN.search(content)
        sec_match = SECTION_PATTERN.search(content)
        self._fallback_store.append(
            {
                "document_id": document_id,
                "title": title,
                "section": sec_match.group(0) if sec_match else "§1.0",
                "content": content,
                "spare_part": part_match.group(1) if part_match else None,
                "tenant_id": tenant_id,
            }
        )
