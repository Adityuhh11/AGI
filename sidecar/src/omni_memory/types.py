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
    VIDEO = "video"  # a video found by its name: the text is its path
    VIDEO_FRAME = "video_frame"  # one frame of a video, embedded as a picture
    AUDIO = "audio"  # an audio file found by its name: the text is its path
    AUDIO_WINDOW = (
        "audio_window"  # 30 s of sound (an audio file or a video's soundtrack), embedded as audio
    )
    FILE = "file"  # an Office file or an archive found by its name: the text is its path


@dataclass(frozen=True, slots=True)
class IndexOptions:
    """What a person chose to index, per kind of file.

    `pdf_pages`: "auto" embeds a page's image when it looks visual, "all" every page, "text" none.
    `photos`: "content" embeds the pixels, "name" only the file name. `videos`: "name" the file
    name, "content" also eight frames. `audio`: "name" the file name, "content" also what is said
    and played, in audio files and in the soundtrack of videos.
    """

    pdf_pages: str = "auto"
    photos: str = "content"
    videos: str = "name"
    audio: str = "name"

    def __post_init__(self) -> None:
        for name, value, allowed in (
            ("pdf_pages", self.pdf_pages, ("auto", "all", "text")),
            ("photos", self.photos, ("content", "name")),
            ("videos", self.videos, ("name", "content")),
            ("audio", self.audio, ("name", "content")),
        ):
            if value not in allowed:
                raise ValueError(f"{name} must be one of {', '.join(allowed)}, got {value!r}")


# What an index built before the options existed did: every PDF page, photos by what they show.
# Videos were not indexed at all, so whatever is chosen for them only adds files; audio files were
# found by name as plain files, which the first sync after the upgrade redoes (see the indexer).
LEGACY_OPTIONS = IndexOptions(pdf_pages="all", photos="content", videos="name", audio="name")


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
    source_id: str
    uri: str
    kind: ChunkKind
    locator: str
    score: float  # cosine + small keyword and path boosts: orders hits, not a probability
    vector_score: (
        float | None
    )  # cosine, an audio window's shifted (see Searcher); None if not returned
    keyword_rank: int | None  # 1-based bm25 rank; None if the keyword search did not return it
    snippet: str


@dataclass(frozen=True, slots=True)
class SearchResult:
    """What a search returns: the ranked hits and whether any of them looks like an answer."""

    hits: list[SearchHit]
    confident: bool  # False: the best hit is probably noise, so "nothing relevant" is the answer


@dataclass(frozen=True, slots=True)
class FileHit:
    """One file in a file-level search: where it is and the chunks of it that matched best."""

    source_id: str
    uri: str
    kind: ChunkKind  # the kind of the best chunk (a PDF is `pdf_text` or `pdf_page`)
    score: float  # the best chunk's score
    chunks: list[SearchHit]  # best first; at most `per_file`


@dataclass(frozen=True, slots=True)
class FileSearchResult:
    files: list[FileHit]
    confident: bool


@dataclass(frozen=True, slots=True)
class SourceInfo:
    """One indexed source: where it came from, when it was last scanned and how much it holds."""

    id: str
    root: str
    last_scan_ns: int  # 0 until the first sync finishes
    files: int
    chunks: int
