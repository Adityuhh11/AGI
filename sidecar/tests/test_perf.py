import time

import numpy as np
import pytest

from agi_memory.embedding import FakeEmbedder
from agi_memory.search.searcher import Searcher
from agi_memory.store import FlatStore
from agi_memory.types import Chunk, ChunkKind

pytestmark = pytest.mark.perf

N = 100_000
DIM = 256
BATCH = 1000
VOCAB = [f"word{i}" for i in range(3000)]


def test_hybrid_search_over_100k_chunks_stays_under_50_ms_p95(tmp_path):
    rng = np.random.default_rng(7)
    embedder = FakeEmbedder(DIM)
    store = FlatStore(tmp_path / "idx", model_id=embedder.model_id, dim=DIM)
    try:
        for start in range(0, N, BATCH):
            chunks = [
                Chunk(
                    "perf", f"f{start + i}.txt", ChunkKind.TEXT, "{}", f"h{start + i}", 1,
                    f"f{start + i}.txt\n" + " ".join(rng.choice(VOCAB, 30)),
                )
                for i in range(BATCH)
            ]  # fmt: skip
            raw = rng.standard_normal((BATCH, DIM)).astype(np.float32)
            store.upsert(chunks, raw / np.linalg.norm(raw, axis=1, keepdims=True))
        searcher = Searcher(store, embedder)
        queries = [" ".join(rng.choice(VOCAB, 4)) for _ in range(200)]
        searcher.search(queries[0])  # warm up
        rounds = []  # best of three: a laptop's background load can triple one round's timings
        for _ in range(3):
            timings = []
            for query in queries:
                started = time.perf_counter()
                searcher.search(query)
                timings.append((time.perf_counter() - started) * 1000)
            rounds.append(tuple(np.percentile(timings, [50, 95])))
        print(
            "100k chunks, (p50, p95) ms per round: "
            + ", ".join(f"({a:.1f}, {b:.1f})" for a, b in rounds)
        )
        assert min(p95 for _, p95 in rounds) < 50
    finally:
        store.close()
