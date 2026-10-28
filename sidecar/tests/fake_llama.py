"""A stand-in for llama-server's HTTP side, for testing LlamaEmbedder without a model.

It speaks the parts the embedder uses (`/tokenize`, `/v1/embeddings`), checks the API key,
records every
request, counts TCP connections, and can drop a number of requests to play a server that died.
"""

from __future__ import annotations

import base64
import io
import json
import threading
import wave
import zlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import numpy as np
from PIL import Image

DIM = 768
API_KEY = "test-key"


def tokenize(text: str) -> list[int]:
    """One id per word between an opening (2) and a closing (1) token, like the real shape."""
    return [2, *(10 + zlib.crc32(word.encode()) % 5000 for word in text.split()), 1]


def vector(seed_text: str) -> list[float]:
    rng = np.random.default_rng(zlib.crc32(seed_text.encode()))
    return rng.normal(size=DIM).tolist()


def expected_for_text(text: str) -> list[float]:
    """What the fake answers for a short text sent as a string."""
    return vector("text:" + text)


def expected_for_ids(ids: list[int]) -> list[float]:
    return vector("ids:" + ",".join(map(str, ids)))


class FakeLlama:
    def __init__(self) -> None:
        self.api_key = API_KEY
        self.requests: list[tuple[str, dict]] = []
        self.client_ports: set[int] = set()
        self.drop_next = 0  # close this many requests without an answer
        self.gpu_lost = False  # answer 500 "ErrorDeviceLost", as the real server does
        self.loading = False  # answer 503 "Loading model", as the real server does while it starts
        self.image_sizes: list[tuple[int, int]] = []
        self.audio_clips: list[tuple[int, int, int]] = []  # (rate, channels, frames) of each WAV
        owner = self

        class Handler(BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"  # keep-alive

            def log_message(self, *args) -> None:
                pass

            def do_POST(self) -> None:
                body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                owner.client_ports.add(self.client_address[1])
                if self.headers.get("Authorization") != "Bearer " + owner.api_key:
                    return self._send(401, {"error": "no"})
                if owner.loading:
                    return self._send(503, {"error": "Loading model"})
                if owner.gpu_lost:
                    return self._send(
                        500, {"error": "decode() failed: vk::Queue::submit: ErrorDeviceLost"}
                    )
                if owner.drop_next > 0:
                    owner.drop_next -= 1
                    self.close_connection = True
                    return None
                owner.requests.append((self.path, body))
                if self.path == "/tokenize":
                    return self._send(200, {"tokens": tokenize(body["content"])})
                return self._send(
                    200,
                    {
                        "data": [
                            {"index": i, "embedding": owner._embed(item)}
                            for i, item in enumerate(body["input"])
                        ]
                    },
                )

            def _send(self, status: int, payload: dict) -> None:
                data = json.dumps(payload).encode()
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

        self._server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.url = f"http://127.0.0.1:{self._server.server_address[1]}"
        threading.Thread(target=self._server.serve_forever, daemon=True).start()

    def _embed(self, item) -> list[float]:
        if isinstance(item, str):
            return expected_for_text(item)
        if isinstance(item, list):
            return expected_for_ids(item)
        part = item["content"][0]
        if part["type"] == "input_audio":
            data = part["input_audio"]["data"]
            with wave.open(io.BytesIO(base64.b64decode(data))) as clip:
                self.audio_clips.append(
                    (clip.getframerate(), clip.getnchannels(), clip.getnframes())
                )
            return vector("audio:" + data[-64:])
        uri = part["image_url"]["url"]
        image = Image.open(io.BytesIO(base64.b64decode(uri.split(",", 1)[1])))
        self.image_sizes.append(image.size)
        return vector("image:" + uri[-64:])

    def close(self) -> None:
        self._server.shutdown()
        self._server.server_close()


class Handle:
    """The `Server` the embedder is given: a url, a key, and a restart that is counted."""

    def __init__(self, fake: FakeLlama, device: str = "vulkan") -> None:
        self.fake, self.url, self.api_key, self.device = fake, fake.url, API_KEY, device
        self.restarts = 0

    def restart(self) -> None:
        """A new server has a new key, as the real one does."""
        self.restarts += 1
        self.api_key = self.fake.api_key = f"key-{self.restarts + 1}"

    def fall_back_to_cpu(self) -> None:
        self.device = "cpu"
        self.fake.gpu_lost = False
        self.restart()
