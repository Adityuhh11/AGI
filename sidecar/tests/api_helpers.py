"""Shared pieces for the job and HTTP API tests: embedders that misbehave on purpose."""

import threading
import time

from agi_memory.embedding import FakeEmbedder

DIM = 64


class GatedEmbedder(FakeEmbedder):
    """Holds an index job open: `embed_documents` blocks until `release` is set."""

    def __init__(self) -> None:
        super().__init__(DIM)
        self.entered = threading.Event()
        self.release = threading.Event()

    def embed_documents(self, texts):
        self.entered.set()
        assert self.release.wait(10), "the gate was never released"
        return super().embed_documents(texts)


class FailingEmbedder(FakeEmbedder):
    def __init__(self) -> None:
        super().__init__(DIM)

    def embed_documents(self, texts):
        raise RuntimeError("gpu on fire")


class SlowEmbedder(FakeEmbedder):
    """Makes each embedding call take a visible amount of time."""

    def __init__(self) -> None:
        super().__init__(DIM)

    def embed_documents(self, texts):
        time.sleep(0.03)
        return super().embed_documents(texts)
