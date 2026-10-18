"""S1/S2 on the owner's real index: re-embed a random sample of stored chunks and compare to the
vectors torch stored. Reads a COPY of the index (meta.db + vectors.f32), never the live one.

    python -m spikes.s5_real <index-copy-dir> <server-url>
"""

import random
import sqlite3
import sys
from pathlib import Path

import numpy as np
import pypdfium2  # noqa: F401  (PdfPages needs it)

from agi_memory.chunking.image import load_image
from agi_memory.chunking.pdf import PdfPages
from agi_memory.config import Settings
from spikes.llama_embedder import LlamaEmbedder

directory, url = Path(sys.argv[1]), sys.argv[2]
settings = Settings(index_dir=directory)
db = sqlite3.connect(f"file:{directory / 'meta.db'}?mode=ro", uri=True)
roots = dict(db.execute("select id, root from sources"))
dim = 768
vectors = np.memmap(directory / "vectors.f32", dtype=np.uint8, mode="r")


def stored(row: int) -> np.ndarray:
    start = 64 + row * dim * 4
    return np.frombuffer(vectors[start : start + dim * 4], dtype=np.float32)


def sample(kinds: tuple[str, ...], n: int):
    marks = ",".join("?" * len(kinds))
    rows = db.execute(
        f"select source_id, uri, kind, locator, text, vec_row from chunks where kind in ({marks})", kinds
    ).fetchall()
    random.Random(7).shuffle(rows)
    return rows[:n]


def stats(label: str, cos: list[float], skipped: int = 0) -> None:
    c = np.asarray(cos)
    print(f"{label:10} n {len(c):4}  min {c.min():.5f}  p1 {np.percentile(c, 1):.5f}  "
          f"p5 {np.percentile(c, 5):.5f}  median {np.median(c):.5f}  mean {c.mean():.5f}  skipped {skipped}")


embedder = LlamaEmbedder(url)

for kinds, n in ((("code",), 150), (("markdown",), 150), (("text",), 100), (("pdf_text",), 150)):
    rows = sample(kinds, n)
    texts = [r[4] for r in rows]
    titles = [r[1].rsplit("/", 1)[-1] for r in rows]
    got = embedder.embed_documents(texts, titles)
    stats(kinds[0], [float(got[i] @ stored(r[5])) for i, r in enumerate(rows)])

cos, skipped = [], 0
for source_id, uri, _kind, _loc, _text, vec_row in sample(("image",), 160):
    try:
        data = (Path(roots[source_id]) / uri).read_bytes()
        image = load_image(data, settings.image_max_side)
    except Exception:
        skipped += 1
        continue
    cos.append(float(embedder.embed_images([image])[0] @ stored(vec_row)))
stats("image", cos, skipped)

cos, skipped = [], 0
for source_id, uri, _kind, locator, _text, vec_row in sample(("pdf_page",), 200):
    page = int(locator.split(":")[1].strip("}"))
    try:
        with PdfPages((Path(roots[source_id]) / uri).read_bytes()) as pdf:
            image = pdf.render(page - 1, settings.pdf_render_scale)
    except Exception:
        skipped += 1
        continue
    cos.append(float(embedder.embed_images([image])[0] @ stored(vec_row)))
stats("pdf_page", cos, skipped)
