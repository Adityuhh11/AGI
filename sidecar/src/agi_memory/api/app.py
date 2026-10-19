"""Application factory: wires the store, the background model load, auth and the host check.

The model takes ~10 s to load. The store opens at once and the model loads on a daemon thread,
so the port answers `/health` immediately (`ready: false`) and the launcher can show progress
instead of waiting on a closed port. Every other route answers 503 until the engine exists.
"""

from __future__ import annotations

import contextlib
import logging
import threading
from collections.abc import AsyncIterator, Callable

from fastapi import Depends, FastAPI
from starlette.middleware.trustedhost import TrustedHostMiddleware

from agi_memory.api.deps import Engine, token_guard
from agi_memory.api.routes import router
from agi_memory.config import Settings
from agi_memory.embedding.base import Embedder
from agi_memory.embedding.serialized import SerializedEmbedder
from agi_memory.indexer import Indexer
from agi_memory.jobs import JobManager
from agi_memory.pace import PacedEmbedder, Pacer
from agi_memory.search.searcher import Searcher
from agi_memory.store import FlatStore

logger = logging.getLogger(__name__)
# uvicorn's own logger, so the line shows in the process output the launcher keeps
progress = logging.getLogger("uvicorn.error")

MIN_TOKEN_CHARS = 16
# Anything else (a web page resolving its own name to 127.0.0.1) is a DNS-rebinding attempt.
ALLOWED_HOSTS = ["127.0.0.1", "localhost"]


def _calibrate(searcher: Searcher) -> None:
    ceiling = searcher.calibrate()
    progress.info(
        "confidence threshold: %s",
        f"{ceiling:.3f} (measured on this index)" if ceiling else "default (index too small)",
    )


def _load_engine(
    app: FastAPI, store: FlatStore, settings: Settings, make_embedder: Callable[[], Embedder]
) -> None:
    try:
        store.warm()  # while the model loads: the first searches should not be the slow ones
        embedder = SerializedEmbedder(make_embedder())
        if (embedder.model_id, embedder.dim) != (settings.model_id, settings.dim):
            raise RuntimeError(
                f"embedder is {embedder.model_id} dim {embedder.dim} but the index is "
                f"{settings.model_id} dim {settings.dim}"
            )
        searcher = Searcher(store, embedder)
        _calibrate(searcher)  # before the engine is announced, so no search sees the default
        pacer = Pacer()
        app.state.engine = Engine(
            embedder=embedder,
            searcher=searcher,
            pacer=pacer,
            # Only indexing is paced; searches use the embedder directly and never wait.
            jobs=JobManager(
                Indexer(store, PacedEmbedder(embedder, pacer), settings),
                on_done=lambda: _calibrate(searcher),
            ),
        )
    except Exception as error:  # report through /health instead of killing the process
        logger.exception("loading the embedding model failed")
        app.state.load_error = f"{type(error).__name__}: {error}"


def create_app(settings: Settings, token: str, make_embedder: Callable[[], Embedder]) -> FastAPI:
    if len(token) < MIN_TOKEN_CHARS:
        raise ValueError(f"the API token must have at least {MIN_TOKEN_CHARS} characters")

    @contextlib.asynccontextmanager
    async def lifespan(app: FastAPI) -> AsyncIterator[None]:
        store = FlatStore(settings.index_dir, model_id=settings.model_id, dim=settings.dim)
        app.state.store = store
        threading.Thread(
            target=_load_engine,
            args=(app, store, settings, make_embedder),
            name="model-loader",
            daemon=True,
        ).start()
        try:
            yield
        finally:
            store.close()

    app = FastAPI(
        title="AGI memory engine",
        lifespan=lifespan,
        docs_url=None,
        redoc_url=None,
        openapi_url=None,
    )
    app.state.settings = settings
    app.state.engine = None
    app.state.load_error = None
    app.include_router(router, dependencies=[Depends(token_guard(token))])
    # Added last, so it runs first: a foreign Host is refused before the token is even read.
    app.add_middleware(TrustedHostMiddleware, allowed_hosts=ALLOWED_HOSTS)
    return app
