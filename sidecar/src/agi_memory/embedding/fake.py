"""Deterministic stand-in for the real model, for tests that must not need a GPU.

Text becomes a hashed bag of words: every token owns a fixed pseudo-random direction and a
text is the normalised sum of its tokens' directions. Texts that share words therefore have
high cosine similarity, which is enough structure to exercise search code, and the same text
always yields the same vector. It is not semantic: synonyms are unrelated.
"""

from __future__ import annotations

import hashlib
import re
from collections.abc import Sequence
from functools import cache

import numpy as np
from PIL.Image import Image

_TOKEN = re.compile(r"[a-z0-9]+")
_EMPTY = "<empty>"


@cache
def _direction(token: str, dim: int) -> np.ndarray:
    seed = int.from_bytes(hashlib.sha256(token.encode()).digest()[:8], "little")
    vec = np.random.default_rng(seed).standard_normal(dim).astype(np.float32)
    vec /= np.linalg.norm(vec)
    vec.setflags(write=False)  # cached and shared: callers must not mutate it
    return vec


class FakeEmbedder:
    """Implements `Embedder` with hashed bags of words (text) and content hashes (images)."""

    def __init__(self, dim: int = 256) -> None:
        self._dim = dim

    @property
    def model_id(self) -> str:
        return "fake-hashed-bow-v1"

    @property
    def dim(self) -> int:
        return self._dim

    @property
    def device(self) -> str:
        return "cpu"

    def _embed_tokens(self, tokens: list[str]) -> np.ndarray:
        if not tokens:
            return _direction(_EMPTY, self._dim).copy()
        total = np.sum([_direction(t, self._dim) for t in tokens], axis=0)
        norm = np.linalg.norm(total)
        if norm == 0:  # tokens cancelled out exactly; vanishingly unlikely but must stay unit
            return _direction(_EMPTY, self._dim).copy()
        return (total / norm).astype(np.float32)

    def _embed_text(self, text: str) -> np.ndarray:
        return self._embed_tokens(_TOKEN.findall(text.lower()))

    def _stack(self, rows: list[np.ndarray]) -> np.ndarray:
        if not rows:
            return np.zeros((0, self._dim), dtype=np.float32)
        return np.stack(rows).astype(np.float32)

    def embed_documents(
        self, texts: Sequence[str], titles: Sequence[str] | None = None
    ) -> np.ndarray:
        if titles is None:
            return self._stack([self._embed_text(t) for t in texts])
        if len(titles) != len(texts):
            raise ValueError(f"{len(titles)} titles for {len(texts)} texts")
        return self._stack(
            [self._embed_text(f"{u} {t}") for u, t in zip(titles, texts, strict=True)]
        )

    def embed_images(self, images: Sequence[Image]) -> np.ndarray:
        rows = []
        for image in images:
            digest = hashlib.sha256(f"{image.mode}{image.size}".encode() + image.tobytes())
            rows.append(self._embed_tokens([f"img:{digest.hexdigest()}"]))
        return self._stack(rows)

    def embed_query(self, text: str) -> np.ndarray:
        return self._embed_text(text)
