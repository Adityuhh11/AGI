"""Shared pieces for the job and HTTP API tests: embedders that misbehave on purpose."""

import contextlib
import threading
import time
from collections.abc import Callable, Iterator
from pathlib import Path

from fastapi.testclient import TestClient

from agi_memory.api.app import create_app
from agi_memory.config import Settings
from agi_memory.embedding import FakeEmbedder
from agi_memory.embedding.base import Embedder
from helpers import wait_for

DIM = 64
TOKEN = "t0ken-" + "x" * 26
AUTH = {"Authorization": f"Bearer {TOKEN}"}
BASE = "http://127.0.0.1:7861"


class GatedEmbedder(FakeEmbedder):
    """Holds an index job open: `embed_documents` blocks until `release` is set."""

    def __init__(self) -> None:
        super().__init__(DIM)
        self.entered = threading.Event()
        self.release = threading.Event()

    def embed_documents(self, texts, titles=None):
        self.entered.set()
        assert self.release.wait(10), "the gate was never released"
        return super().embed_documents(texts, titles)


class FailingEmbedder(FakeEmbedder):
    def __init__(self) -> None:
        super().__init__(DIM)

    def embed_documents(self, texts, titles=None):
        raise RuntimeError("gpu on fire")


class SlowEmbedder(FakeEmbedder):
    """Makes each embedding call take a visible amount of time."""

    def __init__(self) -> None:
        super().__init__(DIM)

    def embed_documents(self, texts, titles=None):
        time.sleep(0.03)
        return super().embed_documents(texts, titles)


def wait_ready(client: TestClient) -> None:
    def check() -> bool:
        body = client.get("/health").json()
        assert body["error"] is None, body["error"]
        return body["ready"]

    wait_for(check)


def index(client: TestClient, folder: Path, source_id: str = "repo") -> dict:
    """Add a folder and wait for its job to finish; returns the finished job as JSON."""
    response = client.post("/sources", json={"path": str(folder), "source_id": source_id})
    assert response.status_code == 202, response.text
    job_id = response.json()["job_id"]
    return wait_for(lambda: (j := client.get(f"/jobs/{job_id}").json())["state"] != "running" and j)


@contextlib.contextmanager
def running_app(
    index_dir: Path,
    make_embedder: Callable[[], Embedder] = lambda: FakeEmbedder(DIM),
    *,
    wait: bool = True,
    **settings_overrides,
) -> Iterator[TestClient]:
    """A live app (lifespan running) with the token preset on the client."""
    probe = FakeEmbedder(DIM)
    settings = Settings(index_dir=index_dir, model_id=probe.model_id, dim=DIM, **settings_overrides)
    app = create_app(settings, TOKEN, make_embedder)
    with TestClient(app, base_url=BASE, headers=AUTH) as client:
        if wait:
            wait_ready(client)
        yield client
