"""Where "a real match" starts on this index, measured instead of assumed.

The best cosine of a query that nothing answers is not a constant: the more chunks there are, the
better the best of them happens to resemble even nonsense. A fixed threshold tuned on the 431-chunk
sample folder passed 71% of nonsense queries on a 44,000-chunk index. So the index measures its own
noise ceiling: it runs a fixed set of nonsense probe queries, and a real query is confident when its
best cosine is above where nine in ten probes ended. Measured against a fixed cosine and against
per-query z-scores in docs/eval/2026-10-16-confidence-gate-experiments.md.
"""

from __future__ import annotations

import random

from omni_memory.embedding.base import Embedder
from omni_memory.store import FlatStore

PROBE_COUNT = 60
CEILING_PERCENTILE = 0.9
MIN_PROBES = 10  # fewer answers than this (a near-empty index) is too little to measure
# Below this the index is too small for its best-of-N noise to say anything about a real corpus;
# the default threshold, tuned on a 431-chunk folder and so on the cautious side here, stays.
MIN_CHUNKS = 100

# Ordinary words with no connection to each other, so a probe is a query nothing answers.
_WORDS = [
    "window",
    "banana",
    "cathedral",
    "seven",
    "river",
    "lantern",
    "marble",
    "thunder",
    "pocket",
    "violin",
    "orchard",
    "glacier",
    "copper",
    "whistle",
    "meadow",
    "anchor",
    "velvet",
    "saddle",
    "harbor",
    "quartz",
    "ember",
    "falcon",
    "mosaic",
    "tundra",
    "bishop",
    "compass",
    "thistle",
    "walrus",
    "cobweb",
    "sundial",
    "paddle",
    "mitten",
    "lagoon",
    "rhubarb",
    "turnip",
    "beacon",
    "fiddle",
    "pebble",
    "cricket",
    "saffron",
    "trellis",
    "gondola",
    "monsoon",
    "bramble",
    "cinder",
    "ferry",
    "lattice",
    "nutmeg",
    "oyster",
]


def probe_queries(count: int = PROBE_COUNT, seed: int = 7) -> list[str]:
    """Half are three unrelated words, half are random letters; the same list every time."""
    rng = random.Random(seed)
    queries = [" ".join(rng.sample(_WORDS, 3)) for _ in range(count // 2)]
    while len(queries) < count:
        letters = (rng.choice("abcdefghijklmnopqrstuvwxyz") for _ in range(rng.randint(6, 11)))
        queries.append("".join(letters))
    return queries


def noise_ceiling(store: FlatStore, embedder: Embedder) -> float | None:
    """The cosine that `CEILING_PERCENTILE` of nonsense probes stayed under, or None if the
    index is too small to say."""
    if store.count() < MIN_CHUNKS:
        return None
    best = []
    for query in probe_queries():
        hits = store.search_vectors(embedder.embed_query(query), 1)
        if hits:
            best.append(hits[0].score)
    if len(best) < MIN_PROBES:
        return None
    best.sort()
    return best[min(int(CEILING_PERCENTILE * len(best)), len(best) - 1)]
