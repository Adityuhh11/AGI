"""Lets several threads share one embedder, with search queries served ahead of indexing.

The API answers search requests while an index job embeds documents, both on the same model.
Neither the GPU model nor its tokenizer is guaranteed re-entrant, so calls take turns. Bulk calls
(documents, images) are cut into small slices and the lock is taken per slice; a query that
arrives waits for the slice in flight, not for the whole call, and queued queries run before the
next slice starts. Measured on the sample folder: a search during indexing took up to 1.3 s when
a bulk call held the lock for its whole duration.
"""

from __future__ import annotations

import threading
from collections.abc import Callable, Sequence
from typing import TYPE_CHECKING

import numpy as np

from omni_memory.embedding.base import Embedder

if TYPE_CHECKING:
    from PIL.Image import Image


class SerializedEmbedder:
    # Two images per slice: `LlamaEmbedder` has two in flight at once, so a query still waits for
    # about one image's time.
    def __init__(self, inner: Embedder, *, text_slice: int = 4, image_slice: int = 2) -> None:
        self._inner = inner
        self._text_slice = text_slice
        self._image_slice = image_slice
        self._lock = threading.Lock()
        self._queries_waiting = 0
        self._no_query_waiting = threading.Condition()

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
        if titles is not None and len(titles) != len(texts):
            return self._inner.embed_documents(texts, titles)  # the embedder reports the mismatch

        def embed(start: int, stop: int) -> np.ndarray:
            part = None if titles is None else titles[start:stop]
            return self._inner.embed_documents(texts[start:stop], part)

        return self._in_slices(len(texts), self._text_slice, embed)

    def embed_images(self, images: Sequence[Image]) -> np.ndarray:
        def embed(start: int, stop: int) -> np.ndarray:
            return self._inner.embed_images(images[start:stop])

        return self._in_slices(len(images), self._image_slice, embed)

    def embed_query(self, text: str) -> np.ndarray:
        with self._no_query_waiting:
            self._queries_waiting += 1
        try:
            with self._lock:
                return self._inner.embed_query(text)
        finally:
            with self._no_query_waiting:
                self._queries_waiting -= 1
                self._no_query_waiting.notify_all()

    def _in_slices(
        self, count: int, size: int, embed: Callable[[int, int], np.ndarray]
    ) -> np.ndarray:
        if count <= size:
            return self._one_slice(embed, 0, count)
        starts = range(0, count, size)
        return np.concatenate([self._one_slice(embed, s, min(s + size, count)) for s in starts])

    def _one_slice(
        self, embed: Callable[[int, int], np.ndarray], start: int, stop: int
    ) -> np.ndarray:
        with self._no_query_waiting:
            self._no_query_waiting.wait_for(lambda: self._queries_waiting == 0)
        with self._lock:
            return embed(start, stop)
