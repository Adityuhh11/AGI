"""S1/S2 step 1: index Sample Data with the torch embedder and record every input and vector.

    python spikes/s1_record.py <out.pkl>
"""

import io
import pickle
import sys
import tempfile
from collections.abc import Sequence
from pathlib import Path

import numpy as np

from agi_memory.config import Settings
from agi_memory.embedding.gemma import GemmaEmbedder
from agi_memory.eval.golden import load_golden
from agi_memory.indexer import Indexer
from agi_memory.store import FlatStore


class Recorder:
    def __init__(self, inner: GemmaEmbedder) -> None:
        self.inner = inner
        self.docs: list[tuple[str, str | None, np.ndarray]] = []
        self.images: list[tuple[bytes, np.ndarray]] = []
        model_id, dim, device = inner.model_id, inner.dim, inner.device
        self.model_id, self.dim, self.device = model_id, dim, device

    def embed_documents(self, texts: Sequence[str], titles: Sequence[str] | None = None):
        out = self.inner.embed_documents(texts, titles)
        for i, text in enumerate(texts):
            self.docs.append((text, titles[i] if titles else None, out[i]))
        return out

    def embed_images(self, images):
        out = self.inner.embed_images(images)
        for image, vec in zip(images, out, strict=True):
            buffer = io.BytesIO()
            image.convert("RGB").save(buffer, "PNG")
            self.images.append((buffer.getvalue(), vec))
        return out

    def embed_query(self, text: str):
        return self.inner.embed_query(text)


settings = Settings(index_dir=Path(tempfile.mkdtemp()))
recorder = Recorder(GemmaEmbedder(settings))
with tempfile.TemporaryDirectory() as work:
    store = FlatStore(Path(work) / "index", model_id=recorder.model_id, dim=recorder.dim)
    report = Indexer(store, recorder, settings).sync("sample", Path("../Sample Data"))
    store.close()
queries = [(item.query, recorder.embed_query(item.query)) for item in load_golden(Path("eval/golden.yaml"))]
print(f"docs {len(recorder.docs)}, images {len(recorder.images)}, queries {len(queries)}, errors {len(report.errors)}")
with open(sys.argv[1], "wb") as out:
    pickle.dump({"docs": recorder.docs, "images": recorder.images, "queries": queries}, out)
