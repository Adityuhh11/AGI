"""Audio windows sit lower against text than the other kinds do, so they are calibrated apart and
shifted onto the shared scale (docs/eval/2026-10-28-audio-embedding.md, section 7)."""

import numpy as np
import pytest

from helpers import file_record, make_chunk
from omni_memory.embedding import FakeEmbedder
from omni_memory.search.calibration import (
    AUDIO_KINDS,
    DEFAULT_AUDIO_CEILING,
    MIN_AUDIO_WINDOWS,
    noise_ceiling,
)
from omni_memory.search.searcher import Searcher
from omni_memory.store import FlatStore
from omni_memory.types import Chunk, ChunkKind

DIM = 16


class Fixed:
    """Embeds every query as the first axis."""

    model_id, dim, device = "fixed", DIM, "cpu"

    def embed_query(self, text):
        out = np.zeros(DIM, dtype=np.float32)
        out[0] = 1.0
        return out


def at_cosine(cosine: float, axis: int) -> np.ndarray:
    out = np.zeros(DIM, dtype=np.float32)
    out[0], out[axis] = cosine, np.sqrt(1 - cosine**2)
    return out


def audio_chunk(uri: str, start: float) -> Chunk:
    return Chunk("s", uri, ChunkKind.AUDIO_WINDOW, f'{{"t":{start}}}', f"{uri}:{start}", 1, "")


@pytest.fixture
def store(tmp_path):
    flat = FlatStore(tmp_path / "idx", model_id="fixed", dim=DIM)
    yield flat
    flat.close()


def put(store, chunk, vector):
    store.replace_file(file_record(chunk.uri), [chunk], vector[None, :])


def test_a_kind_offset_moves_only_the_chunks_of_that_kind(store):
    put(store, make_chunk("text"), at_cosine(0.60, 1))
    put(store, audio_chunk("clip.wav", 0.0), at_cosine(0.50, 2))
    query = Fixed().embed_query("x")
    plain = [(h.id, round(h.score, 3)) for h in store.search_vectors(query, 5)]
    shifted = store.search_vectors(query, 5, None, {ChunkKind.AUDIO_WINDOW: 0.15})
    audio, text = (c.id for c in (audio_chunk("clip.wav", 0.0), make_chunk("text")))
    assert [i for i, _ in plain] == [text, audio]
    assert [(h.id, round(h.score, 3)) for h in shifted] == [(audio, 0.65), (text, 0.60)]
    assert store.search_vectors(query, 5, None, {ChunkKind.AUDIO_WINDOW: 0.0})[0].id == text


def test_chunks_are_counted_by_kind_and_the_kind_lookup_never_reads_the_chunk_rows(store):
    put(store, make_chunk("text"), at_cosine(0.6, 1))
    put(store, audio_chunk("clip.wav", 0.0), at_cosine(0.5, 2))
    assert (store.count(), store.count(AUDIO_KINDS), store.count({ChunkKind.IMAGE})) == (2, 1, 0)
    plan = store._db.execute(
        "EXPLAIN QUERY PLAN SELECT vec_row FROM chunks WHERE kind = ?", ("audio_window",)
    ).fetchall()
    assert "COVERING INDEX chunks_kind" in " ".join(row[-1] for row in plan)


def test_an_audio_hit_below_the_shared_threshold_is_confident_on_its_own_scale(store):
    # raw cosine 0.62: under the 0.70 ceiling of text, over the 0.60 ceiling of audio windows
    put(store, audio_chunk("clip.wav", 0.0), at_cosine(0.62, 2))
    searcher = Searcher(store, Fixed(), confident_cosine=0.70)  # type: ignore[arg-type]
    assert searcher.audio_offset == pytest.approx(0.70 - DEFAULT_AUDIO_CEILING)
    result = searcher.search("zzz")
    assert result.confident and result.hits[0].vector_score == pytest.approx(0.72, abs=1e-6)


def test_a_text_hit_at_the_same_cosine_is_not_confident(store):
    put(store, make_chunk("text"), at_cosine(0.62, 1))
    result = Searcher(store, Fixed(), confident_cosine=0.70).search("zzz")  # type: ignore[arg-type]
    assert not result.confident and result.hits[0].vector_score == pytest.approx(0.62, abs=1e-6)


def test_noise_ceiling_can_be_measured_over_some_kinds_and_needs_enough_of_them(store):
    for i in range(12):
        put(store, audio_chunk(f"c{i}.wav", 0.0), at_cosine(0.4, 1 + i % 10))
    embedder = FakeEmbedder(DIM)
    assert noise_ceiling(store, embedder) is None  # 12 chunks in all: under the 100 of the default
    assert noise_ceiling(store, embedder, AUDIO_KINDS, MIN_AUDIO_WINDOWS) is not None
    assert noise_ceiling(store, embedder, AUDIO_KINDS, 13) is None
    assert noise_ceiling(store, embedder, {ChunkKind.IMAGE}, 1) is None  # none of that kind


def test_calibration_measures_audio_apart_and_shifts_it_up_to_the_others(store):
    rng = np.random.default_rng(5)

    def random_unit(n):
        raw = rng.standard_normal((n, DIM)).astype(np.float32)
        return raw / np.linalg.norm(raw, axis=1, keepdims=True)

    for i, vec in enumerate(random_unit(150)):  # a library of text, the larger pool
        put(store, make_chunk(f"t{i}"), vec)
    embedder = FakeEmbedder(DIM)
    searcher = Searcher(store, embedder)
    before = searcher.calibrate()
    assert before == noise_ceiling(store, embedder)  # no audio: unchanged from before this feature
    for i, vec in enumerate(random_unit(20)):
        put(store, audio_chunk(f"a{i}.wav", 0.0), vec)
    text_only = noise_ceiling(store, embedder, frozenset(ChunkKind) - AUDIO_KINDS)
    audio_only = noise_ceiling(store, embedder, AUDIO_KINDS, MIN_AUDIO_WINDOWS)
    assert searcher.calibrate() == text_only
    assert searcher.audio_offset == pytest.approx(text_only - audio_only)
    assert searcher.audio_offset > 0  # fewer windows than chunks: a lower best-of-N noise
