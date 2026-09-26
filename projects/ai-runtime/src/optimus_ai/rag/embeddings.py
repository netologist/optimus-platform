from __future__ import annotations

import hashlib
import re
from typing import Sequence

import numpy as np


def compute_embedding(text: str, dim: int = 384) -> list[float]:
    """Compute a deterministic, normalized vector embedding for text.

    Uses a fast multi-hash projection with sub-word n-grams and token weighting,
    producing a normalized 384-dimensional vector suitable for pgvector (HNSW cosine similarity).
    Operates with zero external network calls, zero API costs, and sub-millisecond execution.
    """
    if not text or not text.strip():
        return [0.0] * dim

    clean_text = text.lower().strip()
    tokens = re.findall(r"\b\w+\b", clean_text)
    if not tokens:
        tokens = [clean_text]

    vector = np.zeros(dim, dtype=np.float32)

    # 1. Project individual tokens and character n-grams into the embedding space
    for i, token in enumerate(tokens):
        # Position-weighted token hash
        pos_weight = 1.0 / (1.0 + 0.05 * i)
        token_hash = int(hashlib.sha256(token.encode("utf-8")).hexdigest()[:8], 16)
        idx = token_hash % dim
        sign = 1.0 if (token_hash % 2 == 0) else -1.0
        vector[idx] += sign * 1.5 * pos_weight

        # Character trigrams for morphological and technical term capture (e.g. P-104, COOL-4021)
        if len(token) >= 3:
            for j in range(len(token) - 2):
                ngram = token[j : j + 3]
                ng_hash = int(hashlib.md5(ngram.encode("utf-8")).hexdigest()[:8], 16)
                ng_idx = ng_hash % dim
                ng_sign = 1.0 if (ng_hash % 2 == 0) else -1.0
                vector[ng_idx] += ng_sign * 0.5

    # 2. Overall phrase fingerprint
    phrase_hash = int(hashlib.sha256(clean_text.encode("utf-8")).hexdigest()[:16], 16)
    for k in range(8):
        h = (phrase_hash >> (k * 8)) & 0xFF
        vector[(h * 7) % dim] += 0.3

    # 3. L2 Normalization for Cosine Distance (<=> in pgvector)
    norm = np.linalg.norm(vector)
    if norm > 0:
        vector = vector / norm

    return [float(x) for x in vector]


def compute_embeddings_batch(texts: Sequence[str], dim: int = 384) -> list[list[float]]:
    """Compute embeddings for a batch of text chunks."""
    return [compute_embedding(t, dim=dim) for t in texts]
