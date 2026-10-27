import numpy as np
import pytest

from omni_memory.search.calibration import (
    MIN_CHUNKS,
    MIN_PROBES,
    PROBE_COUNT,
    noise_ceiling,
    probe_queries,
)
from omni_memory.types import VectorHit


def test_the_probes_are_the_same_unrelated_queries_every_time():
    probes = probe_queries()
    assert probes == probe_queries()
    assert len(probes) == len(set(probes)) == PROBE_COUNT
    assert all(query.strip() for query in probes)


class _Embedder:
    def embed_query(self, query):
        return np.zeros(4, dtype=np.float32)


class _Store:
    """Answers the n-th probe with the n-th scripted best cosine; None means no hit."""

    def __init__(self, scores, chunks=1000):
        self._scores = iter(scores)
        self._chunks = chunks

    def count(self):
        return self._chunks

    def search_vectors(self, vec, n):
        score = next(self._scores)
        return [] if score is None else [VectorHit(id="x", score=score)]


def test_the_ceiling_is_where_nine_in_ten_probes_ended():
    scores = [i / 100 for i in range(PROBE_COUNT)]  # 0.00 ... 0.59, in any order
    ceiling = noise_ceiling(_Store(reversed(scores)), _Embedder())
    assert ceiling == pytest.approx(scores[int(0.9 * PROBE_COUNT)])


def test_a_probe_that_finds_nothing_is_left_out():
    scores = [None, None] + [0.5] * (PROBE_COUNT - 2)
    assert noise_ceiling(_Store(scores), _Embedder()) == pytest.approx(0.5)


def test_an_index_too_small_to_answer_the_probes_has_no_ceiling():
    scores = [0.5] * (MIN_PROBES - 1) + [None] * (PROBE_COUNT - MIN_PROBES + 1)
    assert noise_ceiling(_Store(scores), _Embedder()) is None


def test_an_index_with_too_few_chunks_is_not_measured_at_all():
    assert noise_ceiling(_Store([0.5] * PROBE_COUNT, chunks=MIN_CHUNKS - 1), _Embedder()) is None
    assert noise_ceiling(_Store([0.5] * PROBE_COUNT, chunks=MIN_CHUNKS), _Embedder()) == 0.5
