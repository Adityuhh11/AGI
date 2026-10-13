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
    HealthOut,
    JobOut,
    RemovedOut,
    SearchHitOut,
    SearchRequest,
    SearchResponse,
    SourceAccepted,
)
from agi_memory.jobs import Job, JobBusyError
from agi_memory.store import FlatStore
from agi_memory.types import Filters

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


@router.post("/search")
def search(body: SearchRequest, engine: Annotated[Engine, Depends(get_engine)]) -> SearchResponse:
    filters = None
    if body.filters is not None:
        filters = Filters(
            kinds=None if body.filters.kinds is None else frozenset(body.filters.kinds),
            source_id=body.filters.source_id,
            uri_prefix=body.filters.uri_prefix,
            modified_after_ns=body.filters.modified_after_ns,
        )
    hits = engine.searcher.search(body.query, body.k, filters)
    return SearchResponse(hits=[SearchHitOut.model_validate(hit) for hit in hits])
