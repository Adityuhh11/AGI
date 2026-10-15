"""Request and response bodies. Unknown request fields are errors, not silently ignored."""

from __future__ import annotations

from pydantic import BaseModel, ConfigDict, Field

from agi_memory.jobs import JobState
from agi_memory.types import ChunkKind

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


class AddSource(_Request):
    path: str = Field(min_length=1, max_length=1024)
    source_id: str = Field(pattern=SOURCE_ID_PATTERN)


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
