"""Spike: the `Embedder` contract on top of a running llama-server (EmbeddingGemma 2 GGUF)."""

from __future__ import annotations

import base64
import http.client
import io
import json
import math
import urllib.parse
from collections.abc import Sequence

import numpy as np
from PIL import Image as PILImage
from PIL.Image import Image

_SHORT = 400  # a text this short has at most 400 tokens, so it never needs truncating (and a tokenize call)


class LlamaEmbedder:
    def __init__(
        self,
        url: str = "http://127.0.0.1:8089",
        *,
        model_id: str = "google/embeddinggemma-2@llamacpp",
        dim: int = 768,
        max_seq_length: int = 512,
        image_format: str = "PNG",
        batch: int = 8,
        fit_images: bool = True,
    ) -> None:
        parts = urllib.parse.urlsplit(url)
        self._host, self._port = parts.hostname, parts.port
        self._conn: http.client.HTTPConnection | None = None
        self._model_id, self._dim, self._fit = model_id, dim, fit_images
        self._max, self._image_format, self._batch = max_seq_length, image_format, batch

    model_id = property(lambda self: self._model_id)
    dim = property(lambda self: self._dim)
    device = property(lambda self: "llama.cpp")

    def _post(self, path: str, body: dict) -> dict:
        """One keep-alive connection; a dropped one is reopened once."""
        payload = json.dumps(body).encode()
        for attempt in (0, 1):
            try:
                if self._conn is None:
                    self._conn = http.client.HTTPConnection(self._host, self._port, timeout=300)
                self._conn.request("POST", path, payload, {"Content-Type": "application/json"})
                return json.load(self._conn.getresponse())
            except (OSError, http.client.HTTPException):
                self._conn = None
                if attempt:
                    raise
        raise AssertionError("unreachable")

    def _vectors(self, inputs: list) -> np.ndarray:
        data = self._post("/v1/embeddings", {"input": inputs})["data"]
        rows = [None] * len(inputs)
        for item in data:
            rows[item["index"]] = item["embedding"]
        vecs = np.asarray(rows, dtype=np.float32)
        return vecs / np.linalg.norm(vecs, axis=1, keepdims=True)

    def _text_inputs(self, texts: Sequence[str]) -> list:
        """Strings stay strings; a long one becomes its token ids cut to the model's limit,
        keeping the closing token exactly like the HF tokenizer's truncation does."""
        out: list = []
        for text in texts:
            if len(text) <= _SHORT:
                out.append(text)
                continue
            ids = self._post("/tokenize", {"content": text, "add_special": True})["tokens"]
            out.append(ids if len(ids) <= self._max else ids[: self._max - 1] + ids[-1:])
        return out

    def _embed_texts(self, texts: Sequence[str]) -> np.ndarray:
        if len(texts) == 0:
            return np.zeros((0, self._dim), dtype=np.float32)
        inputs = self._text_inputs(texts)
        # One request cannot mix strings and token lists, so each kind goes in its own.
        parts = []
        for want_ids in (False, True):
            idx = [i for i, item in enumerate(inputs) if isinstance(item, list) == want_ids]
            for start in range(0, len(idx), self._batch):
                chunk = idx[start : start + self._batch]
                parts.append((chunk, self._vectors([inputs[i] for i in chunk])))
        out = np.zeros((len(texts), self._dim), dtype=np.float32)
        for chunk, vecs in parts:
            out[chunk] = vecs
        return out

    def embed_documents(
        self, texts: Sequence[str], titles: Sequence[str] | None = None
    ) -> np.ndarray:
        if titles is None:
            return self._embed_texts([f"title: none | text: {t}" for t in texts])
        if len(titles) != len(texts):
            raise ValueError(f"{len(titles)} titles for {len(texts)} texts")
        return self._embed_texts([f"title: {u} | text: {t}" for u, t in zip(titles, texts, strict=True)])

    def embed_query(self, text: str) -> np.ndarray:
        return self._embed_texts([f"task: search result | query: {text}"])[0]

    @staticmethod
    def _fitted(image: Image) -> Image:
        """The size the model's own processor gives an image: about 2520 patches of 16 px, aspect kept,
        each side a multiple of 48. Handing llama.cpp that size leaves it nothing to resize."""
        width, height = image.size
        scale = math.sqrt(2520 * 256 / (width * height))
        size = (max(48, int(width * scale / 48) * 48), max(48, int(height * scale / 48) * 48))
        return image if size == image.size else image.resize(size, PILImage.Resampling.BICUBIC)

    def _data_uri(self, image: Image) -> str:
        if self._fit:
            image = self._fitted(image.convert("RGB"))
        buffer = io.BytesIO()
        image.convert("RGB").save(buffer, self._image_format)
        mime = "image/png" if self._image_format == "PNG" else "image/jpeg"
        return f"data:{mime};base64," + base64.b64encode(buffer.getvalue()).decode()

    def embed_images(self, images: Sequence[Image]) -> np.ndarray:
        if len(images) == 0:
            return np.zeros((0, self._dim), dtype=np.float32)
        rows = []
        for start in range(0, len(images), self._batch):
            part = images[start : start + self._batch]
            inputs = [
                {"content": [{"type": "image_url", "image_url": {"url": self._data_uri(i)}}]}
                for i in part
            ]
            rows.append(self._vectors(inputs))
        return np.concatenate(rows)
