"""S4: throughput and query latency on the recorded inputs.

    python -m spikes.s4_speed <torch-record.pkl> torch
    python -m spikes.s4_speed <torch-record.pkl> http://127.0.0.1:8089 [batch]
"""

import io
import pickle
import sys
import tempfile
import time
from pathlib import Path

import numpy as np
from PIL import Image

record = pickle.load(open(sys.argv[1], "rb"))
target = sys.argv[2]
texts = [d[0] for d in record["docs"]]
titles = [d[1] for d in record["docs"]]
images = [Image.open(io.BytesIO(b)) for b, _ in record["images"]]
queries = [q for q, _ in record["queries"]]

if target == "torch":
    from agi_memory.config import Settings
    from agi_memory.embedding.gemma import GemmaEmbedder

    embedder = GemmaEmbedder(Settings(index_dir=Path(tempfile.mkdtemp())))
else:
    from spikes.llama_embedder import LlamaEmbedder

    embedder = LlamaEmbedder(target, batch=int(sys.argv[3]) if len(sys.argv) > 3 else 8)

for q in queries[:5]:  # warm up
    embedder.embed_query(q)
embedder.embed_documents(texts[:16], titles[:16])
embedder.embed_images(images[:4])

start = time.perf_counter()
embedder.embed_documents(texts, titles)
docs_s = time.perf_counter() - start
start = time.perf_counter()
embedder.embed_images(images)
images_s = time.perf_counter() - start
times = []
for _ in range(2):
    for q in queries:
        start = time.perf_counter()
        embedder.embed_query(q)
        times.append((time.perf_counter() - start) * 1000)
p50, p95 = np.percentile(times, [50, 95])
print(f"{target}: {len(texts) / docs_s:.0f} chunks/s ({docs_s:.1f} s for {len(texts)}), "
      f"{len(images) / images_s:.1f} images/s ({images_s:.1f} s for {len(images)}), "
      f"query p50 {p50:.1f} ms p95 {p95:.1f} ms")
