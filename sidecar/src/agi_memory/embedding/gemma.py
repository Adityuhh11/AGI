"""EmbeddingGemma 2 through sentence-transformers.

On the GPU the model runs in bfloat16 (about 0.8 GiB resident and 1.3 GiB peak at the batch
sizes in `Settings` on the dev laptop); on the CPU it runs in float32. Output vectors are
truncated to `Settings.dim` (Matryoshka) and L2-normalised in float32.
"""

from __future__ import annotations

import logging
from collections.abc import Sequence
from typing import TYPE_CHECKING, Any

import numpy as np
import torch
from sentence_transformers import SentenceTransformer

from agi_memory.config import Settings
from agi_memory.embedding.query_graph import QueryGraphs

logger = logging.getLogger(__name__)

if TYPE_CHECKING:
    from PIL.Image import Image


class GemmaEmbedder:
    """Implements `Embedder` with EmbeddingGemma 2 (text + vision; audio is a later spec)."""

    def __init__(self, settings: Settings, *, device: str | None = None) -> None:
        self._settings = settings
        self._device = device or ("cuda" if torch.cuda.is_available() else "cpu")
        dtype = torch.bfloat16 if self._device == "cuda" else torch.float32
        self._model = SentenceTransformer(
            settings.model_id,
            device=self._device,
            model_kwargs={"torch_dtype": dtype},
            config_kwargs={"audio_config": None},
        )
        self._model.max_seq_length = settings.max_seq_length
        # Queries go through CUDA graphs on the GPU (6.8 ms instead of 54); anything that goes wrong
        # leaves the normal path, which always works.
        self._graphs: QueryGraphs | None = None
        prompt = self._model.prompts.get("SearchQuery")
        if self._device == "cuda" and prompt:
            try:
                self._graphs = QueryGraphs(self._model, self.dim, prompt)
            except Exception:
                logger.warning(
                    "query CUDA graphs unavailable; queries use the normal path", exc_info=True
                )

    @property
    def model_id(self) -> str:
        return self._settings.model_id

    @property
    def dim(self) -> int:
        return self._settings.dim

    @property
    def device(self) -> str:
        return self._device

    def _encode(
        self, inputs: Sequence[Any], *, prompt_name: str | None, batch_size: int
    ) -> np.ndarray:
        if len(inputs) == 0:
            return np.zeros((0, self.dim), dtype=np.float32)
        raw = self._model.encode(
            list(inputs),
            prompt_name=prompt_name,
            batch_size=batch_size,
            truncate_dim=self.dim,
            normalize_embeddings=True,
            show_progress_bar=False,
        )
        vecs = np.asarray(raw, dtype=np.float32)
        # bfloat16 normalisation is only accurate to ~1e-2; the store requires unit rows.
        return vecs / np.linalg.norm(vecs, axis=1, keepdims=True)

    def embed_documents(
        self, texts: Sequence[str], titles: Sequence[str] | None = None
    ) -> np.ndarray:
        batch_size = self._settings.text_batch_size
        if titles is None:
            return self._encode(texts, prompt_name="Document", batch_size=batch_size)
        if len(titles) != len(texts):
            raise ValueError(f"{len(titles)} titles for {len(texts)} texts")
        # The stock "Document" prompt is `title: none | text: `; fill the title slot instead.
        formatted = [f"title: {u} | text: {t}" for u, t in zip(titles, texts, strict=True)]
        return self._encode(formatted, prompt_name=None, batch_size=batch_size)

    def embed_images(self, images: Sequence[Image]) -> np.ndarray:
        return self._encode(
            [{"image": image} for image in images],
            prompt_name=None,
            batch_size=self._settings.image_batch_size,
        )

    def embed_query(self, text: str) -> np.ndarray:
        if self._graphs is not None:
            try:
                vec = self._graphs.embed(text)
            except Exception:
                logger.warning(
                    "query CUDA graph failed; using the normal path from now on", exc_info=True
                )
                self._graphs = None
            else:
                if vec is not None:
                    return vec
        return self._encode([text], prompt_name="SearchQuery", batch_size=1)[0]
