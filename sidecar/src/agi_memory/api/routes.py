"""The API routes. Auth and host checks are applied around them in `create_app`."""

from __future__ import annotations

import os
from importlib.metadata import version
from pathlib import Path
from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Request
from fastapi import Path as PathParam

from agi_memory.api.deps import Engine, get_engine, get_store
from agi_memory.api.schemas import (
    SOURCE_ID_PATTERN,
    AddSource,
    FileSearchRequest,
    FileSearchResponse,
    HealthOut,
    JobOut,
    RemovedOut,
    SearchFilters,
    SearchHitOut,
    SearchRequest,
    SearchResponse,
    SourceAccepted,
    SourceOut,
)
from agi_memory.jobs import Job, JobBusyError
from agi_memory.store import FlatStore
from agi_memory.types import Filters, SourceInfo

router = APIRouter()

_VERSION = version("agi-sidecar")


@router.get("/health", response_model=HealthOut)
def health(request: Request) -> HealthOut:
    state = request.app.state
    engine = state.engine
    return HealthOut(
        ready=engine is not None,
        model=state.settings.model_id,
        device=None if engine is None else engine.embedder.device,
        embedded_items=state.store.count(),
        version=_VERSION,
        error=state.load_error,
    )


@router.get("/sources", response_model=list[SourceOut])
def list_sources(store: Annotated[FlatStore, Depends(get_store)]) -> list[SourceInfo]:
    return store.sources()


@router.post("/sources", status_code=202)
def add_source(
    body: AddSource,
    engine: Annotated[Engine, Depends(get_engine)],
    store: Annotated[FlatStore, Depends(get_store)],
) -> SourceAccepted:
    root = Path(body.path)
    # A UNC path would make Windows contact another machine, so refuse it before is_dir().
    if not root.is_absolute() or root.drive.startswith("\\\\") or not root.is_dir():
        raise HTTPException(
            status_code=400, detail="path must be an absolute local path to an existing folder"
        )
    indexed_from = store.source_root(body.source_id)
    if indexed_from is not None and os.path.normcase(indexed_from) != os.path.normcase(str(root)):
        raise HTTPException(
            status_code=409,
            detail={
                "message": f"{body.source_id} is indexed from {indexed_from}; delete it first",
                "job_id": None,
            },
        )
    try:
        job = engine.jobs.submit(body.source_id, root)
    except JobBusyError as busy:
        raise HTTPException(
            status_code=409, detail={"message": str(busy), "job_id": busy.job_id}
        ) from busy
    return SourceAccepted(job_id=job.id)


@router.get("/jobs/{job_id}", response_model=JobOut)
def get_job(job_id: str, engine: Annotated[Engine, Depends(get_engine)]) -> Job:
    job = engine.jobs.get(job_id)
    if job is None:
        raise HTTPException(status_code=404, detail=f"no such job: {job_id}")
    return job


@router.delete("/sources/{source_id}")
def delete_source(
    source_id: Annotated[str, PathParam(pattern=SOURCE_ID_PATTERN)],
    engine: Annotated[Engine, Depends(get_engine)],
    store: Annotated[FlatStore, Depends(get_store)],
) -> RemovedOut:
    try:
        removed = engine.jobs.run_unless_indexing(source_id, lambda: store.delete_source(source_id))
    except JobBusyError as busy:
        raise HTTPException(status_code=409, detail=f"{source_id} is being indexed") from busy
    return RemovedOut(removed_chunks=removed)


def _filters(requested: SearchFilters | None) -> Filters | None:
    if requested is None:
        return None
    return Filters(
        kinds=None if requested.kinds is None else frozenset(requested.kinds),
        source_id=requested.source_id,
        uri_prefix=requested.uri_prefix,
        modified_after_ns=requested.modified_after_ns,
    )


@router.post("/search")
def search(body: SearchRequest, engine: Annotated[Engine, Depends(get_engine)]) -> SearchResponse:
    result = engine.searcher.search(body.query, body.k, _filters(body.filters))
    return SearchResponse(
        hits=[SearchHitOut.model_validate(hit) for hit in result.hits], confident=result.confident
    )


@router.post("/search/files")
def search_files(
    body: FileSearchRequest, engine: Annotated[Engine, Depends(get_engine)]
) -> FileSearchResponse:
    result = engine.searcher.search_files(body.query, body.k, body.per_file, _filters(body.filters))
    return FileSearchResponse.model_validate(result)
