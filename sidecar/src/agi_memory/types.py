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
