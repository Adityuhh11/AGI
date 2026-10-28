"""Request and response bodies. Unknown request fields are errors, not silently ignored."""

from __future__ import annotations

from typing import Annotated, Self

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

from omni_memory.jobs import JobState
from omni_memory.pace import MIN_DUTY
from omni_memory.types import ChunkKind, IndexOptions

# Also validated for the path parameter of DELETE /sources/{source_id}.
SOURCE_ID_PATTERN = r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$"


class _Request(BaseModel):
    model_config = ConfigDict(extra="forbid")


class _Response(BaseModel):
    model_config = ConfigDict(from_attributes=True)


class HealthOut(BaseModel):
    ready: bool
    model: str
    device: str | None
    embedded_items: int
    version: str
    error: str | None
    # How hard indexing works and whether it is paused; null until the engine is ready.
    duty: float | None = None
    paused: bool | None = None


class PaceOut(BaseModel):
    duty: float
    paused: bool


class SetPace(_Request):
    duty: float = Field(ge=MIN_DUTY, le=1.0)


_DEFAULT_OPTIONS = IndexOptions()


class IndexOptionsIn(_Request):
    """What to index of each kind of file. The allowed values and the defaults are those of
    `types.IndexOptions`, which checks them; one it refuses is a 422."""

    pdf_pages: str = _DEFAULT_OPTIONS.pdf_pages
    photos: str = _DEFAULT_OPTIONS.photos
    videos: str = _DEFAULT_OPTIONS.videos
    audio: str = _DEFAULT_OPTIONS.audio

    @model_validator(mode="after")
    def _known_values(self) -> Self:
        IndexOptions(**self.model_dump())
        return self


def _relative_folders(folders: list[str]) -> list[str]:
    for folder in folders:
        parts = folder.replace("\\", "/").split("/")
        if folder.startswith(("/", "\\")) or ":" in folder or ".." in parts:
            raise ValueError(f"{folder!r} must be a relative folder inside the source")
    return folders


class AddSource(_Request):
    path: str = Field(min_length=1, max_length=1024)
    source_id: str = Field(pattern=SOURCE_ID_PATTERN)
    options: IndexOptionsIn = Field(default_factory=IndexOptionsIn)
    # Folders below `path` to leave out, '/'-separated and relative, e.g. "vendor/skills".
    exclude: list[Annotated[str, Field(min_length=1, max_length=1024)]] = Field(
        default_factory=list, max_length=500
    )

    @field_validator("exclude")
    @classmethod
    def _inside_the_root(cls, folders: list[str]) -> list[str]:
        return _relative_folders(folders)


class SurveyRequest(_Request):
    path: str = Field(min_length=1, max_length=1024)
    exclude: list[Annotated[str, Field(min_length=1, max_length=1024)]] = Field(
        default_factory=list, max_length=500
    )

    @field_validator("exclude")
    @classmethod
    def _inside_the_root(cls, folders: list[str]) -> list[str]:
        return _relative_folders(folders)


class RouteCountOut(_Response):
    files: int
    bytes: int
    # Audio and video: the length of their sound, guessed from their sizes (a survey opens no file).
    seconds: float = 0.0


class SurveyOut(_Response):
    """What indexing a folder would read, counted without opening any file."""

    routes: dict[str, RouteCountOut]
    denied: int
    ignored: int
    unsupported: int
    too_large: int
    unreadable: int


class SourceAccepted(BaseModel):
    job_id: str


class FileErrorOut(_Response):
    uri: str
    message: str


class ReportOut(_Response):
    added: int
    updated: int
    unchanged: int
    removed: int
    reused: int
    denied: int
    ignored: int
    unsupported: int
    too_large: int
    chunks_written: int
    errors: list[FileErrorOut]


class JobOut(_Response):
    id: str
    source_id: str
    state: JobState
    files_done: int
    files_total: int
    report: ReportOut | None
    error: str | None


class SourceOut(_Response):
    id: str
    root: str
    last_scan_ns: int
    files: int
    chunks: int


class RemovedOut(BaseModel):
    removed_chunks: int


class SearchFilters(_Request):
    kinds: list[ChunkKind] | None = None
    source_id: str | None = Field(default=None, pattern=SOURCE_ID_PATTERN)
    uri_prefix: str | None = Field(default=None, max_length=1024)
    modified_after_ns: int | None = Field(default=None, ge=0)


class SearchRequest(_Request):
    query: str = Field(max_length=4000)  # empty is allowed and returns no hits
    k: int = Field(default=10, ge=1, le=100)
    filters: SearchFilters | None = None


class SearchHitOut(_Response):
    id: str
    source_id: str
    uri: str
    kind: ChunkKind
    locator: str
    score: float
    vector_score: float | None
    keyword_rank: int | None
    snippet: str


class SearchResponse(BaseModel):
    hits: list[SearchHitOut]
    confident: bool  # False: nothing here looks like an answer; show "no match", not the list


class FileSearchRequest(_Request):
    query: str = Field(max_length=4000)
    k: int = Field(default=10, ge=1, le=50)  # files
    per_file: int = Field(default=3, ge=1, le=10)  # matching chunks shown per file
    filters: SearchFilters | None = None


class FileOut(_Response):
    source_id: str
    uri: str
    kind: ChunkKind
    score: float
    chunks: list[SearchHitOut]


class FileSearchResponse(_Response):
    files: list[FileOut]
    confident: bool
