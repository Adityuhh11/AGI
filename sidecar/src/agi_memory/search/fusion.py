"""One blended score from the vector and keyword rankings.

Cosine similarity is the base because it means the same thing for text, PDF pages and images.
Rank fusion did not: an image has no keyword text, so it appeared in one list while a text chunk
appeared in two, and the right image lost to files that merely mentioned its words. Keyword search
now only adds a small boost, enough to settle close calls and to rescue exact names the embedding
misses (measured in docs/eval/2026-10-15-search-quality-experiments.md).
"""

from __future__ import annotations

from collections.abc import Sequence

from agi_memory.types import KeywordHit, VectorHit

KEYWORD_WEIGHT = 0.02


def blend(
    vector: Sequence[VectorHit],
    keyword: Sequence[KeywordHit],
    keyword_weight: float = KEYWORD_WEIGHT,
) -> list[tuple[str, float]]:
    """`cosine + keyword_weight * bm25 / best bm25` per chunk, best first, ties by id.

    A chunk found only by keyword search takes the lowest cosine in the vector pool, so it can
    surface without out-ranking anything the vector search was sure about. Repeated ids count
    once, at their first (best) position.
    """
    cosines: dict[str, float] = {}
    for hit in vector:
        cosines.setdefault(hit.id, hit.score)
    floor = min(cosines.values(), default=0.0)

    boosts: dict[str, float] = {}
    best = max((hit.score for hit in keyword), default=0.0)
    if best > 0:
        for hit in keyword:
            boosts.setdefault(hit.id, hit.score / best)

    scores = {
        chunk_id: cosines.get(chunk_id, floor) + keyword_weight * boosts.get(chunk_id, 0.0)
        for chunk_id in cosines.keys() | boosts.keys()
    }
    return sorted(scores.items(), key=lambda item: (-item[1], item[0]))
