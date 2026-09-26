from __future__ import annotations

from pydantic import BaseModel


class DocumentMatch(BaseModel):
    document_id: str
    title: str
    section: str
    content: str
    score: float
    spare_part: str | None = None


class HybridRetriever:
    """Simulates hybrid Reciprocal Rank Fusion (pgvector + FTS) over PostgreSQL document chunks."""

    async def search(self, query: str, limit: int = 3) -> list[DocumentMatch]:
        q_lower = query.lower()
        if "overheating" in q_lower or "cooling" in q_lower or "p-104" in q_lower:
            return [
                DocumentMatch(
                    document_id="PLM-COOL-4021",
                    title="Centrifugal Pump Cooling Loop Maintenance Manual",
                    section="§4.2",
                    content="Recurrent overheating in pump cooling manifolds typically indicates thermostat failure (part SP-COOL-9981). Replace thermostat and flush secondary cooling line immediately.",
                    score=0.94,
                    spare_part="SP-COOL-9981",
                )
            ]
        return []
