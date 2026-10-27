"""Request-scoped dependencies: the loaded engine, the store and the token check."""

from __future__ import annotations

import hmac
from collections.abc import Callable
from dataclasses import dataclass
from typing import Annotated

from fastapi import Header, HTTPException, Request

from omni_memory.embedding.base import Embedder
from omni_memory.jobs import JobManager
from omni_memory.pace import Pacer
from omni_memory.search.searcher import Searcher
from omni_memory.store import FlatStore


@dataclass(frozen=True, slots=True)
class Engine:
    """Everything that needs the embedding model; exists only once the model has loaded."""

    embedder: Embedder
    searcher: Searcher
    jobs: JobManager
    pacer: Pacer


def get_engine(request: Request) -> Engine:
    state = request.app.state
    if state.engine is None:
        reason = "the model is still loading"
        if state.load_error is not None:
            reason = f"the model failed to load: {state.load_error}"
        raise HTTPException(status_code=503, detail=reason)
    return state.engine


def get_store(request: Request) -> FlatStore:
    return request.app.state.store


def token_guard(token: str) -> Callable[..., None]:
    """A dependency that admits only `Authorization: Bearer <token>` (constant-time compare)."""
    expected = f"Bearer {token}".encode()

    def guard(authorization: Annotated[str | None, Header()] = None) -> None:
        if authorization is None or not hmac.compare_digest(authorization.encode(), expected):
            raise HTTPException(
                status_code=401,
                detail="missing or wrong bearer token",
                headers={"WWW-Authenticate": "Bearer"},
            )

    return guard
