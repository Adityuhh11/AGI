"""Request and response bodies. Unknown request fields are errors, not silently ignored."""

from __future__ import annotations

from pydantic import BaseModel, ConfigDict, Field

from agi_memory.jobs import JobState

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


class RemovedOut(BaseModel):
    removed_chunks: int
