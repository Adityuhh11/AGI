"""S1/S2 step 2: embed the recorded inputs with a llama-server and compare to the torch vectors.

    python spikes/s1_compare.py <torch-record.pkl> <server-url> [image-format]
"""

import io
import pickle
import sys

import numpy as np
from PIL import Image

from spikes.llama_embedder import LlamaEmbedder

record = pickle.load(open(sys.argv[1], "rb"))
import os

embedder = LlamaEmbedder(sys.argv[2], image_format=sys.argv[3] if len(sys.argv) > 3 else "PNG", fit_images=os.environ.get("FIT", "1") == "1")


def stats(label: str, cos: np.ndarray) -> None:
    p = np.percentile(cos, [1, 5, 50])
    print(f"{label:9} n {len(cos):4}  min {cos.min():.5f}  p1 {p[0]:.5f}  p5 {p[1]:.5f}  "
          f"median {p[2]:.5f}  mean {cos.mean():.5f}")


texts = [d[0] for d in record["docs"]]
titles = [d[1] for d in record["docs"]]
ref = np.stack([d[2] for d in record["docs"]])
got = embedder.embed_documents(texts, titles)
cos = (ref * got).sum(1)
stats("documents", cos)
for i in np.argsort(cos)[:3]:
    print(f"   worst doc {cos[i]:.5f}  title={titles[i]!r} chars={len(texts[i])}")

images = [Image.open(io.BytesIO(b)) for b, _ in record["images"]]
ref = np.stack([v for _, v in record["images"]])
cos_img = (ref * embedder.embed_images(images)).sum(1)
stats("images", cos_img)
print(f"   worst images {np.sort(cos_img)[:3]}")

ref = np.stack([v for _, v in record["queries"]])
got = np.stack([embedder.embed_query(q) for q, _ in record["queries"]])
stats("queries", (ref * got).sum(1))

# Ranking agreement: documents and images in one corpus, as the engine ranks them.
corpus_ref = np.concatenate([np.stack([d[2] for d in record["docs"]]), np.stack([v for _, v in record["images"]])])
corpus_got = np.concatenate([embedder.embed_documents(texts, titles), embedder.embed_images(images)])
q_ref = np.stack([v for _, v in record["queries"]])
q_got = np.stack([embedder.embed_query(q) for q, _ in record["queries"]])
top_ref = np.argsort(-(q_ref @ corpus_ref.T), axis=1)[:, :10]
top_got = np.argsort(-(q_got @ corpus_got.T), axis=1)[:, :10]
overlap = np.array([len(set(a) & set(b)) / 10 for a, b in zip(top_ref, top_got, strict=True)])
same_top1 = float((top_ref[:, 0] == top_got[:, 0]).mean())
n_docs = len(record["docs"])
img_share_ref = float((top_ref[:, 0] >= n_docs).mean())
img_share_got = float((top_got[:, 0] >= n_docs).mean())
print(f"ranking   top-10 overlap mean {overlap.mean():.3f} (min {overlap.min():.1f}); same top-1 {same_top1:.3f}; "
      f"image at rank 1: torch {img_share_ref:.3f} vs llama {img_share_got:.3f}")
