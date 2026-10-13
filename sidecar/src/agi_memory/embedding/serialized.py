"""Lets several threads share one embedder.

The API serves search requests while an index job embeds documents, both on the same model.
Neither the GPU model nor its tokenizer is guaranteed re-entrant, so calls take turns. A
query therefore waits for at most the batch that is currently running (about 200 ms).
"""

from __future__ import annotations

import threading
from collections.abc import Sequence
from typing import TYPE_CHECKING

import numpy as np

from agi_memory.embedding.base import Embedder

if TYPE_CHECKING:
    from PIL.Image import Image


class SerializedEmbedder:
    def __init__(self, inner: Embedder) -> None:
        self._inner = inner
        self._lock = threading.Lock()

    @property
    def model_id(self) -> str:
        return self._inner.model_id

    @property
    def dim(self) -> int:
        return self._inner.dim

    @property
    def device(self) -> str:
        return self._inner.device

    def embed_documents(self, texts: Sequence[str]) -> np.ndarray:
        with self._lock:
            return self._inner.embed_documents(texts)

    def embed_images(self, images: Sequence[Image]) -> np.ndarray:
        with self._lock:
            return self._inner.embed_images(images)

    def embed_query(self, text: str) -> np.ndarray:
        with self._lock:
            return self._inner.embed_query(text)
