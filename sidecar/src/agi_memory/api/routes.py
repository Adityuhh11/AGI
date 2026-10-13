"""The API routes. Auth and host checks are applied around them in `create_app`."""

from __future__ import annotations

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
def add_source(body: AddSource, engine: Annotated[Engine, Depends(get_engine)]) -> SourceAccepted:
    root = Path(body.path)
    if not root.is_absolute() or not root.is_dir():
        raise HTTPException(
            status_code=400, detail="path must be an absolute path to an existing folder"
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
    if engine.jobs.running_source() == source_id:
        raise HTTPException(status_code=409, detail=f"{source_id} is being indexed")
    return RemovedOut(removed_chunks=store.delete_source(source_id))


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
