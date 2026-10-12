"""Searcher: embeds the query, runs vector and keyword search, fuses and describes the hits."""

from __future__ import annotations

from agi_memory.embedding.base import Embedder
from agi_memory.search.fusion import reciprocal_rank_fusion
from agi_memory.store import FlatStore
from agi_memory.types import Chunk, Filters, SearchHit

_SNIPPET_CHARS = 240


def _snippet(chunk: Chunk) -> str:
    """The chunk body (text after the path line), whitespace collapsed and shortened."""
    _, _, body = chunk.text.partition("\n")
    return " ".join(body.split())[:_SNIPPET_CHARS]


class Searcher:
    """Hybrid search over one store. Query embedding costs ~200 ms; the rest is milliseconds."""

    def __init__(
        self, store: FlatStore, embedder: Embedder, *, pool: int = 200, rrf_k: int = 60
    ) -> None:
        self._store = store
        self._embedder = embedder
        self._pool = pool
        self._rrf_k = rrf_k

    def search(self, query: str, k: int = 10, filters: Filters | None = None) -> list[SearchHit]:
        """Top-`k` chunks for `query`, best first; `[]` for a blank query or `k <= 0`."""
        if not query.strip() or k <= 0:
            return []
        vector_hits = self._store.search_vectors(
            self._embedder.embed_query(query), self._pool, filters
        )
        keyword_hits = self._store.keyword_search(query, self._pool, filters)
        fused = reciprocal_rank_fusion(
            [[h.id for h in vector_hits], [h.id for h in keyword_hits]], self._rrf_k
        )[:k]
        vector_scores = {h.id: h.score for h in vector_hits}
        keyword_ranks = {h.id: rank for rank, h in enumerate(keyword_hits, start=1)}
        chunks = self._store.get([chunk_id for chunk_id, _ in fused])
        hits = []
        for chunk_id, score in fused:
            chunk = chunks.get(chunk_id)
            if chunk is None:  # deleted between ranking and lookup
                continue
            hits.append(
                SearchHit(
                    id=chunk_id,
                    uri=chunk.uri,
                    kind=chunk.kind,
                    locator=chunk.locator,
                    score=score,
                    vector_score=vector_scores.get(chunk_id),
                    keyword_rank=keyword_ranks.get(chunk_id),
                    snippet=_snippet(chunk),
                )
            )
        return hits
