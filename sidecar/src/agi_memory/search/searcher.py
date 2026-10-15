"""Searcher: embeds the query, runs vector, keyword and path search, blends and describes hits."""

from __future__ import annotations

from agi_memory.embedding.base import Embedder
from agi_memory.search.fusion import CONFIDENT_COSINE, blend
from agi_memory.store import FlatStore
from agi_memory.types import Chunk, Filters, SearchHit, SearchResult

_SNIPPET_CHARS = 240


def _snippet(chunk: Chunk) -> str:
    """The chunk body (text after the path line), whitespace collapsed and shortened."""
    _, _, body = chunk.text.partition("\n")
    return " ".join(body.split())[:_SNIPPET_CHARS]


class Searcher:
    """Hybrid search over one store. Query embedding costs ~200 ms; the rest is milliseconds."""

    def __init__(
        self,
        store: FlatStore,
        embedder: Embedder,
        *,
        pool: int = 200,
        confident_cosine: float = CONFIDENT_COSINE,
    ) -> None:
        self._store = store
        self._embedder = embedder
        self._pool = pool
        self._confident_cosine = confident_cosine

    def search(self, query: str, k: int = 10, filters: Filters | None = None) -> SearchResult:
        """Top-`k` chunks for `query`, best first; no hits for a blank query or `k <= 0`.

        `confident` is true when the best hit's cosine reaches `confident_cosine` or the query is
        part of its path (typing a file name is a deliberate match, whatever the cosine says).
        """
        if not query.strip() or k <= 0:
            return SearchResult([], confident=False)
        vector_hits = self._store.search_vectors(
            self._embedder.embed_query(query), self._pool, filters
        )
        keyword_hits = self._store.keyword_search(query, self._pool, filters)
        path_hits = self._store.path_search(query, filters)
        fused = blend(vector_hits, keyword_hits, path_hits)[:k]
        vector_scores = {h.id: h.score for h in vector_hits}
        path_ids = {h.id for h in path_hits}
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
        confident = bool(hits) and (
            hits[0].id in path_ids or (hits[0].vector_score or 0.0) >= self._confident_cosine
        )
        return SearchResult(hits, confident)
