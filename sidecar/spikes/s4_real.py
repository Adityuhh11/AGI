"""S4 at scale: throughput on random chunks, pages and images of the real index (a copy of it).

    python -m spikes.s4_real <index-copy-dir> torch
    python -m spikes.s4_real <index-copy-dir> http://127.0.0.1:8089
Rendering and decoding happen before the clock starts, so only embedding is timed.
"""

import random
import sqlite3
import sys
import tempfile
import time
from pathlib import Path

from agi_memory.chunking.image import load_image
from agi_memory.chunking.pdf import PdfPages
from agi_memory.config import Settings

directory, target = Path(sys.argv[1]), sys.argv[2]
settings = Settings(index_dir=Path(tempfile.mkdtemp()))
db = sqlite3.connect(f"file:{directory / 'meta.db'}?mode=ro", uri=True)
roots = dict(db.execute("select id, root from sources"))


def sample(kinds: tuple[str, ...], n: int):
    marks = ",".join("?" * len(kinds))
    rows = db.execute(
        f"select source_id, uri, locator, text from chunks where kind in ({marks})", kinds
    ).fetchall()
    random.Random(11).shuffle(rows)
    return rows[:n]


text_rows = sample(("code", "markdown", "text", "pdf_text"), 2000)
texts = [r[3] for r in text_rows]
titles = [r[1].rsplit("/", 1)[-1] for r in text_rows]
images = [
    load_image((Path(roots[s]) / u).read_bytes(), settings.image_max_side)
    for s, u, _l, _t in sample(("image",), 100)
]
pages = []
for s, u, loc, _t in sample(("pdf_page",), 150):
    with PdfPages((Path(roots[s]) / u).read_bytes()) as pdf:
        pages.append(pdf.render(int(loc.split(":")[1].strip("}")) - 1, settings.pdf_render_scale))

if target == "torch":
    from agi_memory.embedding.gemma import GemmaEmbedder

    embedder = GemmaEmbedder(settings)
else:
    from spikes.llama_embedder import LlamaEmbedder

    embedder = LlamaEmbedder(target)

embedder.embed_documents(texts[:32], titles[:32])
embedder.embed_images(images[:4])


def timed(label: str, count: int, run) -> None:
    start = time.perf_counter()
    run()
    seconds = time.perf_counter() - start
    print(f"  {label:9} {count / seconds:6.1f} /s  ({seconds:6.1f} s for {count})", flush=True)


print(target)
timed("chunks", len(texts), lambda: embedder.embed_documents(texts, titles))
timed("images", len(images), lambda: embedder.embed_images(images))
timed("pdf pages", len(pages), lambda: embedder.embed_images(pages))
