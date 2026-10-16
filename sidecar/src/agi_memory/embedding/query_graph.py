"""Embeds one search query through CUDA graphs.

The query model is launch-bound: a 15-token query took about 54 ms, nearly all of it Python issuing
roughly a thousand tiny GPU kernels one after another (tokenizing is 0.3 ms). A CUDA graph records
that sequence once per padded length and replays it in one call: 6.8 ms, and the vector matches the
normal path to a cosine of 0.9999 (the same agreement as the GPU and the CPU have with each other).
Measured in docs/eval/2026-10-16-query-latency.md.

Queries are padded (on the right, masked) to the next of a few lengths; a longer query is left to
the normal path, which this class never replaces, only shortcuts.
"""

from __future__ import annotations

import threading
from typing import Any

import numpy as np
import torch
from sentence_transformers import SentenceTransformer

BUCKETS = (16, 32, 64)  # padded token lengths; a typed query is almost always under 16


class QueryGraphs:
    """One captured graph per padded length, sharing the model with the normal path."""

    def __init__(self, model: SentenceTransformer, dim: int, prompt: str) -> None:
        self._model = model
        self._dim = dim
        self._prompt = prompt
        self._lock = threading.Lock()  # the static buffers belong to one replay at a time
        self._graphs = {length: _Graph(model, prompt, length) for length in BUCKETS}

    def embed(self, text: str) -> np.ndarray | None:
        """The query's unit vector, or None when it is too long for any graph."""
        features = self._model.preprocess([self._prompt + text])
        tokens = int(features["input_ids"].shape[1])
        length = next((n for n in BUCKETS if tokens <= n), None)
        if length is None:
            return None
        with self._lock:
            out = self._graphs[length].run(features)
        vec = out[0, : self._dim].float().cpu().numpy()
        return vec / np.linalg.norm(vec)


class _Graph:
    def __init__(self, model: SentenceTransformer, prompt: str, length: int) -> None:
        self._length = length
        self._pad_id = model.tokenizer.pad_token_id or 0
        self._device = next(model.parameters()).device
        warm = model.preprocess([prompt + "warm"])
        self._static: dict[str, Any] = {
            "input_ids": torch.full((1, length), self._pad_id, device=self._device),
            "attention_mask": torch.zeros(
                (1, length), dtype=warm["attention_mask"].dtype, device=self._device
            ),
            **{k: v for k, v in warm.items() if k not in ("input_ids", "attention_mask")},
        }
        self._fill(warm)
        side = torch.cuda.Stream()
        side.wait_stream(torch.cuda.current_stream())
        with torch.cuda.stream(side), torch.inference_mode():
            for _ in range(3):  # lets lazy initialisation finish before recording
                model(dict(self._static))
        torch.cuda.current_stream().wait_stream(side)
        torch.cuda.synchronize()
        self._graph = torch.cuda.CUDAGraph()
        with torch.inference_mode(), torch.cuda.graph(self._graph):
            self._out = model(dict(self._static))["sentence_embedding"]

    def _fill(self, features: dict[str, Any]) -> None:
        ids, mask = features["input_ids"], features["attention_mask"]
        count = ids.shape[1]
        self._static["input_ids"].fill_(self._pad_id)
        self._static["attention_mask"].zero_()
        self._static["input_ids"][0, :count] = ids[0].to(self._device)
        self._static["attention_mask"][0, :count] = mask[0].to(self._device)

    def run(self, features: dict[str, Any]) -> torch.Tensor:
        self._fill(features)
        self._graph.replay()
        return self._out
