"""One blended score from the vector, keyword and path rankings.

Cosine similarity is the base because it means the same thing for text, PDF pages and images.
Rank fusion did not: an image has no keyword text, so it appeared in one list while a text chunk
appeared in two, and the right image lost to files that merely mentioned its words. The other two
signals only add to it: bm25 a small boost that settles close calls and rescues exact names the
embedding misses, and a path match (the query typed as part of a file's path) a larger one.
Weights come from the sweep in docs/eval/2026-10-15-search-quality-experiments.md.
"""

from __future__ import annotations

from collections.abc import Sequence

from omni_memory.types import KeywordHit, VectorHit

KEYWORD_WEIGHT = 0.02
PATH_WEIGHT = 0.2
# Where "a real match" starts until the index has measured its own noise ceiling
# (`search/calibration.py`): on the sample corpus on-topic queries had a top cosine of at least
# 0.674, off-topic ones at most 0.666. It does not hold on a bigger index, hence the calibration.
CONFIDENT_COSINE = 0.65


def blend(
    vector: Sequence[VectorHit],
    keyword: Sequence[KeywordHit],
    path: Sequence[KeywordHit] = (),
    *,
    keyword_weight: float = KEYWORD_WEIGHT,
    path_weight: float = PATH_WEIGHT,
) -> list[tuple[str, float]]:
    """`cosine + keyword_weight * bm25 / best + path_weight * pathscore / best`, best first.

    Each side channel is scaled by its own best score. A chunk found only by a side channel takes
    the lowest cosine in the vector pool, so it can surface without out-ranking anything the
    vector search was sure about. Repeated ids count once, at their first (best) position. Ties
    break by id.
    """
    cosines: dict[str, float] = {}
    for hit in vector:
        cosines.setdefault(hit.id, hit.score)
    floor = min(cosines.values(), default=0.0)
    keyword_boost = _relative(keyword)
    path_boost = _relative(path)

    scores = {
        chunk_id: cosines.get(chunk_id, floor)
        + keyword_weight * keyword_boost.get(chunk_id, 0.0)
        + path_weight * path_boost.get(chunk_id, 0.0)
        for chunk_id in cosines.keys() | keyword_boost.keys() | path_boost.keys()
    }
    return sorted(scores.items(), key=lambda item: (-item[1], item[0]))


def _relative(hits: Sequence[KeywordHit]) -> dict[str, float]:
    """Each id's score as a fraction of the best score; nothing if the best is not positive."""
    best = max((hit.score for hit in hits), default=0.0)
    relative: dict[str, float] = {}
    if best > 0:
        for hit in hits:
            relative.setdefault(hit.id, hit.score / best)
    return relative
