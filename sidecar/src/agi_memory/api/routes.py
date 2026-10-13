"""The API routes. Auth and host checks are applied around them in `create_app`."""

from __future__ import annotations

from importlib.metadata import version

from fastapi import APIRouter, Request

from agi_memory.api.schemas import HealthOut

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
