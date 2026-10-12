"""The Embedder contract: how text and images become unit vectors."""

from __future__ import annotations

from collections.abc import Sequence
from typing import TYPE_CHECKING, Protocol

import numpy as np

if TYPE_CHECKING:
    from PIL.Image import Image


class Embedder(Protocol):
    """Turns content into float32, L2-normalised vectors of length `dim`.

    Documents and queries are separate methods because the real model embeds them with
    different prompts (asymmetric retrieval).
    """

    @property
    def model_id(self) -> str: ...

    @property
    def dim(self) -> int: ...

    def embed_documents(self, texts: Sequence[str]) -> np.ndarray:
        """Shape `(len(texts), dim)`."""
        ...

    def embed_images(self, images: Sequence[Image]) -> np.ndarray:
        """Shape `(len(images), dim)`."""
        ...

    def embed_query(self, text: str) -> np.ndarray:
        """Shape `(dim,)`."""
        ...
