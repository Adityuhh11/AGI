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

    @property
    def device(self) -> str:
        """Where the model runs: `cuda` or `cpu`."""
        ...

    def embed_documents(
        self, texts: Sequence[str], titles: Sequence[str] | None = None
    ) -> np.ndarray:
        """Shape `(len(texts), dim)`. `titles` (one per text, e.g. the file name) is embedded with
        the text when given: the real model has a dedicated title slot that measurably helps."""
        ...

    def embed_images(self, images: Sequence[Image]) -> np.ndarray:
        """Shape `(len(images), dim)`."""
        ...

    def embed_query(self, text: str) -> np.ndarray:
        """Shape `(dim,)`."""
        ...
