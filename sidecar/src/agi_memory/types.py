"""Plain data types shared across the memory engine: no I/O, no behaviour beyond identity."""

from __future__ import annotations

import enum
import hashlib
from dataclasses import dataclass


class ChunkKind(enum.StrEnum):
    """What a chunk's content is; decides how it is embedded and how it is shown."""

    CODE = "code"
    MARKDOWN = "markdown"
    TEXT = "text"
    PDF_TEXT = "pdf_text"
    PDF_PAGE = "pdf_page"
    IMAGE = "image"


def chunk_id(source_id: str, uri: str, locator: str, content_hash: str) -> str:
    """Content-derived id, so re-indexing the same content always yields the same id.

    `source_id` is part of the identity: two sources can hold identical files at the same
    relative path and must not collide.
    """
    return hashlib.sha256(f"{source_id}\0{uri}\0{locator}\0{content_hash}".encode()).hexdigest()


@dataclass(frozen=True, slots=True)
class Chunk:
    """One searchable unit of a source file. Metadata only; vectors live in the store."""

    source_id: str
    uri: str  # path relative to the source root, '/'-separated
    kind: ChunkKind
    locator: str  # JSON text locating the chunk, e.g. '{"lines":[1,40]}' or '{"page":3}'
    content_hash: str  # sha256 hex of the chunk's content
    mtime_ns: int
    text: str  # text indexed for keyword search; '' for images

    @property
    def id(self) -> str:
        return chunk_id(self.source_id, self.uri, self.locator, self.content_hash)


@dataclass(frozen=True, slots=True)
class Filters:
    """Restrictions applied before ranking. `None` means no restriction on that field."""

    kinds: frozenset[ChunkKind] | None = None
    source_id: str | None = None
    uri_prefix: str | None = None
    modified_after_ns: int | None = None


@dataclass(frozen=True, slots=True)
class VectorHit:
    """One result of an exact vector search."""

    id: str
    score: float  # cosine similarity, because stored rows are unit length


@dataclass(frozen=True, slots=True)
class FileRecord:
    """What the index remembers about one indexed file, to detect changes cheaply."""

    source_id: str
    uri: str
    size: int
    mtime_ns: int
    sha256: str


@dataclass(frozen=True, slots=True)
class KeywordHit:
    """One result of a keyword (FTS5/bm25) search."""

    id: str
    score: float  # negated bm25, so higher is better like the vector scores


@dataclass(frozen=True, slots=True)
class SearchHit:
    """One fused search result, with enough detail to show it and to debug the ranking."""

    id: str
    uri: str
    kind: ChunkKind
    locator: str
    score: float  # reciprocal-rank-fusion score (relative, not a probability)
    vector_score: float | None  # cosine similarity; None if the vector search did not return it
    keyword_rank: int | None  # 1-based bm25 rank; None if the keyword search did not return it
    snippet: str


@dataclass(frozen=True, slots=True)
class SearchResult:
    """What a search returns: the ranked hits and whether any of them looks like an answer."""

    hits: list[SearchHit]
    confident: bool  # False: the best hit is probably noise, so "nothing relevant" is the answer
