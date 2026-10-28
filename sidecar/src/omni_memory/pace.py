"""Lets a person slow indexing down or stop it, without touching search.

Indexing keeps the GPU busy for as long as it runs, which is what a person who wants their laptop
for something else does not want. The `Pacer` is told how large a share of the time the indexer may
keep the GPU busy (`duty`: 1.0 is flat out, 0.2 is a fifth of the time) and whether it is paused.
`PacedEmbedder` wraps the embedder the indexer uses: after each bulk embedding call it waits
`busy * (1 / duty - 1)` seconds, so the share of busy time comes out as `duty`, and while paused it
waits until resumed. Queries pass straight through: searching never waits for a person's choice
about indexing.

A pause takes effect when the embedding call in flight returns (a second or two at most); the
indexer records a file only after its vectors are stored, so a pause, or a stop while paused, loses
nothing.
"""

from __future__ import annotations

import threading
import time
from collections.abc import Sequence
from typing import TYPE_CHECKING

import numpy as np

from omni_memory.embedding.base import Embedder

if TYPE_CHECKING:
    from PIL.Image import Image

MIN_DUTY = 0.05  # below this a call would be followed by a wait twenty times as long


class Pacer:
    def __init__(self, duty: float = 1.0) -> None:
        self._cond = threading.Condition()
        self._duty = self._checked(duty)
        self._paused = False

    @staticmethod
    def _checked(duty: float) -> float:
        if not MIN_DUTY <= duty <= 1.0:
            raise ValueError(f"duty must be between {MIN_DUTY} and 1.0, got {duty}")
        return float(duty)

    @property
    def duty(self) -> float:
        return self._duty

    @property
    def paused(self) -> bool:
        return self._paused

    def set_duty(self, duty: float) -> None:
        with self._cond:
            self._duty = self._checked(duty)
            self._cond.notify_all()

    def pause(self) -> None:
        with self._cond:
            self._paused = True
            self._cond.notify_all()

    def resume(self) -> None:
        with self._cond:
            self._paused = False
            self._cond.notify_all()

    def delay_for(self, busy_seconds: float) -> float:
        """The wait that makes `busy_seconds` of work a `duty` share of the time."""
        return max(0.0, busy_seconds) * (1.0 / self._duty - 1.0)

    def checkpoint(self, busy_seconds: float) -> None:
        """After a stretch of work: waits out a pause and the rest period of the duty cycle."""
        started = time.monotonic()
        with self._cond:
            while True:
                if self._paused:
                    self._cond.wait()
                    continue
                # Recomputed each round so a new speed applies to the rest already under way.
                remaining = started + self.delay_for(busy_seconds) - time.monotonic()
                if remaining <= 0:
                    return
                self._cond.wait(remaining)  # a pause or a new speed arriving meanwhile wakes this up


class PacedEmbedder:
    """An `Embedder` that checkpoints with a `Pacer` after each bulk call, never a query."""

    def __init__(self, inner: Embedder, pacer: Pacer) -> None:
        self._inner = inner
        self._pacer = pacer

    @property
    def model_id(self) -> str:
        return self._inner.model_id

    @property
    def dim(self) -> int:
        return self._inner.dim

    @property
    def device(self) -> str:
        return self._inner.device

    def embed_documents(
        self, texts: Sequence[str], titles: Sequence[str] | None = None
    ) -> np.ndarray:
        started = time.monotonic()
        vectors = self._inner.embed_documents(texts, titles)
        self._pacer.checkpoint(time.monotonic() - started)
        return vectors

    def embed_images(self, images: Sequence[Image]) -> np.ndarray:
        started = time.monotonic()
        vectors = self._inner.embed_images(images)
        self._pacer.checkpoint(time.monotonic() - started)
        return vectors

    def embed_query(self, text: str) -> np.ndarray:
        return self._inner.embed_query(text)
