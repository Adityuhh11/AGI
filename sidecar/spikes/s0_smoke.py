"""S0: does llama-server return text and image embeddings for EmbeddingGemma 2? Throwaway."""

import base64
import io
import json
import sys
import urllib.request

import numpy as np
from PIL import Image

URL = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8089"
IMAGE = sys.argv[2] if len(sys.argv) > 2 else "../Sample Data/Virat Kholi.jpg"


def post(path: str, body: dict) -> dict:
    request = urllib.request.Request(
        URL + path, json.dumps(body).encode(), {"Content-Type": "application/json"}
    )
    with urllib.request.urlopen(request, timeout=120) as response:
        return json.load(response)


def report(label: str, response: dict) -> np.ndarray:
    vec = np.asarray(response["data"][0]["embedding"], dtype=np.float32)
    print(f"{label}: dim {vec.shape[0]}, norm {np.linalg.norm(vec):.6f}, first {vec[:3]}")
    return vec


text = post("/v1/embeddings", {"input": ["title: notes.md | text: how to renew a passport"]})
report("text", text)

with Image.open(IMAGE) as picture:
    picture = picture.convert("RGB")
    picture.thumbnail((1024, 1024))
    buffer = io.BytesIO()
    picture.save(buffer, "JPEG")
data_uri = "data:image/jpeg;base64," + base64.b64encode(buffer.getvalue()).decode()
image = post(
    "/v1/embeddings",
    {"input": [{"content": [{"type": "image_url", "image_url": {"url": data_uri}}]}]},
)
report("image", image)
