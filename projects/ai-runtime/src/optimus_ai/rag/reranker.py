from __future__ import annotations

import re
from typing import Sequence

from pydantic import BaseModel


class DocumentMatch(BaseModel):
    document_id: str
    title: str
    section: str
    content: str
    score: float
    spare_part: str | None = None
    rank_type: str = "hybrid"  # "hybrid" | "fts" | "vector" | "in-memory"


class Reranker:
    """Reranker that refines and scores hybrid search candidate matches.

    Applies lexical term weighting, exact asset/part code matching, and structural
    section relevance bonuses to produce high-precision document context for agents.
    """

    def rerank(self, query: str, candidates: Sequence[DocumentMatch], top_k: int = 3) -> list[DocumentMatch]:
        if not candidates:
            return []

        # Extract codes (e.g. SP-FAN-7711, P-104, PLM-COOL-4021) and standard technical words
        codes = {c.lower() for c in re.findall(r"[A-Za-z0-9]+(?:-[A-Za-z0-9]+)+", query)}
        words = {w.lower() for w in re.findall(r"\b[A-Za-z]{3,}\b", query)}
        scored_matches: list[tuple[float, DocumentMatch]] = []

        for candidate in candidates:
            base_score = candidate.score
            cand_text = f"{candidate.document_id} {candidate.title} {candidate.section} {candidate.content}".lower()

            # 1. Exact asset/spare part code match bonus (highest precision signal)
            code_bonus = 0.0
            if codes:
                matched_codes = sum(1 for c in codes if c in cand_text)
                code_bonus = (matched_codes / len(codes)) * 0.4

            # 2. Lexical word overlap bonus
            cand_words = set(re.findall(r"\b[A-Za-z]{3,}\b", cand_text))
            overlap = words.intersection(cand_words)
            lexical_bonus = (len(overlap) / max(len(words), 1)) * 0.35 if words else 0.0

            # 3. Section relevance bonus (maintenance, troubleshooting, fault isolation)
            section_bonus = 0.0
            sec_lower = f"{candidate.section} {candidate.title}".lower()
            if any(term in sec_lower for term in ["maintenance", "manual", "troubleshooting", "failure", "repair", "overhaul", "guide"]):
                section_bonus += 0.1

            # Combined calibrated score
            final_score = min(1.0, base_score * 0.4 + code_bonus + lexical_bonus + section_bonus)

            updated_match = DocumentMatch(
                document_id=candidate.document_id,
                title=candidate.title,
                section=candidate.section,
                content=candidate.content,
                score=round(final_score, 4),
                spare_part=candidate.spare_part,
                rank_type=candidate.rank_type,
            )
            scored_matches.append((final_score, updated_match))

        # Sort descending by final reranked score
        scored_matches.sort(key=lambda x: x[0], reverse=True)
        return [match for _, match in scored_matches[:top_k]]
