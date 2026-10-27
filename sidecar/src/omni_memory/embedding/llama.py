"""EmbeddingGemma 2 through llama.cpp: the `Embedder` contract over a running llama-server.

Measured against the torch model it replaced (docs/eval/2026-10-18-llamacpp-embedder.md): vectors
agree to cosine 0.9999 for text and 0.994 or better for images and PDF pages, and the golden set
gives identical results. What it takes to get there, each found by measuring:

- Texts are the model's own prompts, byte for byte (`title: <name> | text: <text>`,
  `task: search result | query: <q>`).
- The server rejects an input longer than its physical batch, so a long text is tokenized by the
  server, cut to `max_seq_length` tokens keeping the closing token (as the Hugging Face tokenizer's
  truncation does), and sent as token ids.
- Images are resized here to the size the model's own processor uses (about 2520 patches of 16 px,
  aspect kept, each side a multiple of 48, bicubic), so llama.cpp has nothing left to resize;
  without it images drift to cosine 0.97.
- Keep-alive connections: opening a new one per request cost 20 ms a query on Windows.
- Images go `_IMAGE_REQUESTS` at a time, one per request on connections of their own: the server
  decodes and slices one image while the GPU embeds the other (4.4 to 6.4 images/s), and PNG is
  written at compression level 1, half the encoding time of the default for the same pixels
  (docs/eval/2026-10-20-indexing-throughput.md).

Not thread-safe by itself: `SerializedEmbedder` wraps it, as for every embedder.
"""

from __future__ import annotations

import base64
import http.client
import io
import json
import logging
import math
import threading
import urllib.parse
from collections.abc import Sequence
from concurrent.futures import ThreadPoolExecutor
from typing import Protocol

import numpy as np
from PIL import Image as PILImage
from PIL.Image import Image

from omni_memory.config import Settings

logger = logging.getLogger(__name__)

# A text this short has at most 400 tokens, so it never needs cutting (or a tokenize call).
_SHORT_TEXT = 400  # characters
_PATCH_BUDGET = 2520 * 16 * 16  # pixels the model's image processor aims for
_SIDE_STEP = 48  # its patch size (16) times its pooling kernel (3)
_IMAGE_REQUESTS = 2  # measured: 2 in flight 6.4 images/s, 4 in flight 6.5


class Server(Protocol):
    """What the embedder needs of its llama-server: where it is, and a way to start it again."""

    url: str
    api_key: str
    device: str

    def restart(self) -> None: ...

    def fall_back_to_cpu(self) -> None: ...


class LlamaEmbedder:
    """Implements `Embedder` with EmbeddingGemma 2 (text and vision) on a llama-server."""

    def __init__(self, server: Server, settings: Settings) -> None:
        self._server = server
        self._settings = settings
        self._local = threading.local()  # one connection per thread: the image workers have theirs
        self._restarting = threading.Lock()
        self._images = ThreadPoolExecutor(_IMAGE_REQUESTS, thread_name_prefix="embed-image")

    @property
    def model_id(self) -> str:
        return self._settings.model_id

    @property
    def dim(self) -> int:
        return self._settings.dim

    @property
    def device(self) -> str:
        return self._server.device

    def _post(self, path: str, body: dict) -> dict:
        """POST over the open connection, with two ways to recover.

        llama-server closes an idle keep-alive connection after a few seconds, so a failure on the
        first try is most likely that: connect again. A second failure means the server is gone:
        start it again (its url and API key change, so the request is built afresh) and try once
        more. Two image workers can find it gone at once; only the first restarts it, and the other
        waits for that restart to finish before it reads the new address and key.
        """
        payload = json.dumps(body).encode()
        for attempt in range(3):
            # Under the restart lock: a worker that arrives while the other restarts the server
            # waits for it to be healthy, instead of meeting it half loaded or restarting it twice.
            with self._restarting:
                url, key = self._server.url, self._server.api_key
            headers = {"Content-Type": "application/json", "Authorization": "Bearer " + key}
            conn: http.client.HTTPConnection | None = getattr(self._local, "conn", None)
            try:
                if conn is None or conn.port != urllib.parse.urlsplit(url).port:
                    parts = urllib.parse.urlsplit(url)
                    assert parts.hostname is not None  # the server's url is always http://host:port
                    conn = self._local.conn = http.client.HTTPConnection(
                        parts.hostname, parts.port, timeout=300
                    )
                conn.request("POST", path, payload, headers)
                response = conn.getresponse()
                data = response.read()
            except (OSError, http.client.HTTPException) as error:
                self._local.conn = None
                if attempt == 2:
                    raise
                if attempt == 1:
                    with self._restarting:
                        if self._server.api_key == key:  # a restart makes a new key
                            logger.warning(
                                "llama-server did not answer (%s); starting it again", error
                            )
                            self._server.restart()
                continue
            if response.status != 200:
                if attempt < 2 and self._gpu_failed(data):
                    # A graphics card that runs out of memory or is lost (a driver reset, a game
                    # taking the memory) fails every later request too; the CPU is slower but works.
                    with self._restarting:
                        if self._server.api_key == key:
                            logger.warning(
                                "the graphics card failed (%r); continuing on the CPU", data[:120]
                            )
                            self._server.fall_back_to_cpu()
                    continue
                raise RuntimeError(
                    f"llama-server answered {response.status} to {path}: {data[:200]!r}"
                )
            return json.loads(data)
        raise AssertionError("unreachable")

    def _gpu_failed(self, answer: bytes) -> bool:
        return self.device != "cpu" and (
            b"ErrorDeviceLost" in answer or b"ErrorOutOfDeviceMemory" in answer
        )

    def _vectors(self, inputs: list) -> np.ndarray:
        rows: list = [None] * len(inputs)
        for item in self._post("/v1/embeddings", {"input": inputs})["data"]:
            rows[item["index"]] = item["embedding"]
        vecs = np.asarray(rows, dtype=np.float32)[
            :, : self.dim
        ]  # Matryoshka: the first `dim` values
        return vecs / np.linalg.norm(vecs, axis=1, keepdims=True)

    def _text_inputs(self, texts: Sequence[str]) -> list:
        """Short texts as they are; a long one as its token ids, cut to the model's limit."""
        limit = self._settings.max_seq_length
        inputs: list = []
        for text in texts:
            if len(text) <= _SHORT_TEXT:
                inputs.append(text)
                continue
            ids = self._post("/tokenize", {"content": text, "add_special": True})["tokens"]
            inputs.append(ids if len(ids) <= limit else ids[: limit - 1] + ids[-1:])
        return inputs

    def _embed_texts(self, texts: Sequence[str]) -> np.ndarray:
        if len(texts) == 0:
            return np.zeros((0, self.dim), dtype=np.float32)
        inputs = self._text_inputs(texts)
        out = np.zeros((len(texts), self.dim), dtype=np.float32)
        size = self._settings.text_batch_size
        # One request cannot mix strings and token lists, so each kind goes in requests of its own.
        for want_ids in (False, True):
            indexes = [i for i, item in enumerate(inputs) if isinstance(item, list) == want_ids]
            for start in range(0, len(indexes), size):
                chunk = indexes[start : start + size]
                out[chunk] = self._vectors([inputs[i] for i in chunk])
        return out

    def embed_documents(
        self, texts: Sequence[str], titles: Sequence[str] | None = None
    ) -> np.ndarray:
        if titles is None:
            return self._embed_texts([f"title: none | text: {t}" for t in texts])
        if len(titles) != len(texts):
            raise ValueError(f"{len(titles)} titles for {len(texts)} texts")
        return self._embed_texts(
            [f"title: {u} | text: {t}" for u, t in zip(titles, texts, strict=True)]
        )

    def embed_query(self, text: str) -> np.ndarray:
        return self._embed_texts([f"task: search result | query: {text}"])[0]

    @staticmethod
    def _fitted(image: Image) -> Image:
        width, height = image.size
        scale = math.sqrt(_PATCH_BUDGET / (width * height))
        size = (
            max(_SIDE_STEP, int(width * scale / _SIDE_STEP) * _SIDE_STEP),
            max(_SIDE_STEP, int(height * scale / _SIDE_STEP) * _SIDE_STEP),
        )
        return image if size == image.size else image.resize(size, PILImage.Resampling.BICUBIC)

    def _data_uri(self, image: Image) -> str:
        buffer = io.BytesIO()
        # Lossless (JPEG would move the vectors); level 1 halves the encoding time of the default.
        self._fitted(image.convert("RGB")).save(buffer, "PNG", compress_level=1)
        return "data:image/png;base64," + base64.b64encode(buffer.getvalue()).decode()

    def _embed_image(self, image: Image) -> np.ndarray:
        url = self._data_uri(image)
        return self._vectors([{"content": [{"type": "image_url", "image_url": {"url": url}}]}])

    def embed_images(self, images: Sequence[Image]) -> np.ndarray:
        if len(images) == 0:
            return np.zeros((0, self.dim), dtype=np.float32)
        return np.concatenate(list(self._images.map(self._embed_image, images)))
