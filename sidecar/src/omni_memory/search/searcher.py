"""Searcher: embeds the query, runs vector, keyword and path search, blends and describes hits.

Keyword and path search run on a thread of their own while the query is embedded: neither needs
the vector, and both wait on the GPU and SQLite with the GIL released.
"""

from __future__ import annotations

from concurrent.futures import ThreadPoolExecutor

from omni_memory.embedding.base import Embedder
from omni_memory.search.calibration import noise_ceiling
from omni_memory.search.fusion import CONFIDENT_COSINE, blend
from omni_memory.store import FlatStore
from omni_memory.types import Chunk, FileHit, FileSearchResult, Filters, SearchHit, SearchResult

_SNIPPET_CHARS = 240
_MIN_FILE_SEARCH_CHUNKS = 60  # enough chunks that ten files rarely run short


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
        self._lexical = ThreadPoolExecutor(1, thread_name_prefix="search-lexical")

    @property
    def confident_cosine(self) -> float:
        """The best cosine from which a hit counts as a real match."""
        return self._confident_cosine

    def calibrate(self) -> float | None:
        """Measure the index's noise ceiling and make it the confidence threshold; call again when
        the index has changed a lot. Returns the ceiling, or None (threshold unchanged) when the
        index is too small to measure."""
        ceiling = noise_ceiling(self._store, self._embedder)
        if ceiling is not None:
            self._confident_cosine = ceiling
        return ceiling

    def search(self, query: str, k: int = 10, filters: Filters | None = None) -> SearchResult:
        """Top-`k` chunks for `query`, best first; no hits for a blank query or `k <= 0`.

        `confident` is true when the best hit's cosine reaches `confident_cosine` or the query is
        part of its path (typing a file name is a deliberate match, whatever the cosine says).
        """
        if not query.strip() or k <= 0:
            return SearchResult([], confident=False)
        lexical = self._lexical.submit(
            lambda: (
                self._store.keyword_search(query, self._pool, filters),
                self._store.path_search(query, filters),
            )
        )
        vector_hits = self._store.search_vectors(
            self._embedder.embed_query(query), self._pool, filters
        )
        keyword_hits, path_hits = lexical.result()
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
                    source_id=chunk.source_id,
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

    def search_files(
        self,
        query: str,
        files: int = 10,
        per_file: int = 3,
        filters: Filters | None = None,
    ) -> FileSearchResult:
        """The best `files` files for `query`, each with its best `per_file` matching chunks.

        Chunks are ranked as in `search`, then grouped by file in rank order, so a file's place is
        its best chunk's place and a file with many matching chunks does not crowd out the others.
        """
        result = self.search(query, max(_MIN_FILE_SEARCH_CHUNKS, files * per_file), filters)
        grouped: dict[tuple[str, str], list[SearchHit]] = {}
        for hit in result.hits:
            grouped.setdefault((hit.source_id, hit.uri), []).append(hit)
        found = [
            FileHit(source_id, uri, hits[0].kind, hits[0].score, hits[:per_file])
            for (source_id, uri), hits in list(grouped.items())[:files]
        ]
        return FileSearchResult(found, result.confident)
