import threading

import numpy as np
import pytest

from agi_memory.store import FlatStore
from agi_memory.types import ChunkKind, Filters
from helpers import DIM, make_chunk, unit_vectors


def oracle(vectors_by_id: dict[str, np.ndarray], query: np.ndarray, n: int):
    """Plain brute force in float64: the reference every store result is compared to."""
    q = query.astype(np.float64)
    scored = [(float(v.astype(np.float64) @ q), id_) for id_, v in vectors_by_id.items()]
    return sorted(scored, key=lambda item: (-item[0], item[1]))[:n]


def fill(store, count: int, seed: int = 0, **chunk_args):
    chunks = [make_chunk(f"c{i}", **chunk_args) for i in range(count)]
    vectors = unit_vectors(count, seed)
    store.upsert(chunks, vectors)
    return chunks, vectors


def test_matches_the_brute_force_oracle(store):
    chunks, vectors = fill(store, 500)
    by_id = {c.id: v for c, v in zip(chunks, vectors, strict=True)}
    for query in unit_vectors(20, seed=99):
        hits = store.search_vectors(query, 10)
        expected = oracle(by_id, query, 10)
        assert [h.id for h in hits] == [id_ for _, id_ in expected]
        np.testing.assert_allclose([h.score for h in hits], [s for s, _ in expected], atol=1e-5)


def test_empty_store_returns_nothing(store):
    assert store.search_vectors(unit_vectors(1)[0], 10) == []


def test_asking_for_more_than_exists_returns_everything(store):
    chunks, _ = fill(store, 3)
    hits = store.search_vectors(unit_vectors(1, seed=5)[0], 100)
    assert {h.id for h in hits} == {c.id for c in chunks}


def test_non_positive_n_returns_nothing(store):
    fill(store, 3)
    assert store.search_vectors(unit_vectors(1)[0], 0) == []


def test_deleted_chunks_are_not_returned(store):
    chunks, vectors = fill(store, 5)
    store.delete([chunks[0].id])
    hits = store.search_vectors(vectors[0], 10)
    assert chunks[0].id not in {h.id for h in hits}
    assert len(hits) == 4


def test_a_reinserted_id_uses_its_latest_vector(store):
    chunk = make_chunk("a")
    old, new = unit_vectors(2, seed=1)
    store.upsert([chunk], old[None, :])
    store.upsert([chunk], new[None, :])
    (hit,) = store.search_vectors(new, 1)
    assert hit.id == chunk.id
    assert hit.score == pytest.approx(1.0, abs=1e-5)


def test_freed_rows_are_reused_without_resurrecting_old_vectors(store):
    chunks, vectors = fill(store, 3)
    store.delete([chunks[1].id])
    newcomer = make_chunk("new")
    fresh = unit_vectors(1, seed=7)
    store.upsert([newcomer], fresh)
    hits = store.search_vectors(vectors[1], 10)  # the deleted chunk's old vector as the query
    assert chunks[1].id not in {h.id for h in hits}
    assert {h.id for h in hits} == {chunks[0].id, chunks[2].id, newcomer.id}


def test_results_survive_reopen(tmp_path):
    directory = tmp_path / "idx"
    first = FlatStore(directory, model_id="fake", dim=DIM)
    _, vectors = fill(first, 50)
    before = first.search_vectors(vectors[3], 5)
    first.close()

    second = FlatStore(directory, model_id="fake", dim=DIM)
    assert second.search_vectors(vectors[3], 5) == before
    second.close()


def test_ties_are_broken_by_id_whatever_the_insert_order(store, monkeypatch):
    chunks = [make_chunk(name) for name in ("zulu", "alpha", "mike")]
    same = np.repeat(unit_vectors(1), 3, axis=0)
    store.upsert(chunks, same)
    # Identical rows are not always an exact tie: a matrix product may sum a row in another order
    # depending on where it sits and on the processor (a CI runner differed in the last bit). The
    # rule is about exact ties, so make them exact.
    def exact(query, upto):
        return np.ones(upto, dtype=np.float32)

    monkeypatch.setattr(store._vectors, "scores", exact)
    hits = store.search_vectors(same[0], 3)
    assert [h.id for h in hits] == sorted(c.id for c in chunks)
    assert [h.id for h in store.search_vectors(same[0], 2)] == sorted(c.id for c in chunks)[:2]


def test_filters_restrict_candidates(store):
    code = make_chunk("a", kind=ChunkKind.CODE, source_id="repo", mtime_ns=10)
    doc = make_chunk("b", kind=ChunkKind.PDF_TEXT, source_id="docs", mtime_ns=20)
    img = make_chunk("c", kind=ChunkKind.IMAGE, source_id="docs", mtime_ns=30)
    store.upsert([code, doc, img], unit_vectors(3))
    query = unit_vectors(1, seed=2)[0]

    def ids(filters):
        return {h.id for h in store.search_vectors(query, 10, filters)}

    assert ids(None) == {code.id, doc.id, img.id}
    assert ids(Filters(kinds=frozenset({ChunkKind.CODE}))) == {code.id}
    assert ids(Filters(kinds=frozenset())) == set()
    assert ids(Filters(source_id="docs")) == {doc.id, img.id}
    assert ids(Filters(uri_prefix="b")) == {doc.id}
    assert ids(Filters(modified_after_ns=15)) == {doc.id, img.id}
    assert ids(Filters(source_id="docs", modified_after_ns=25)) == {img.id}


@pytest.mark.parametrize("bad", [np.ones(DIM + 1, np.float32), np.full(DIM, np.nan, np.float32)])
def test_bad_queries_are_rejected(store, bad):
    fill(store, 2)
    with pytest.raises(ValueError):
        store.search_vectors(bad, 5)


def test_searching_while_the_vector_file_grows_does_not_crash(store):
    fill(store, 10)
    failures: list[BaseException] = []
    stop = threading.Event()

    def searcher():
        query = unit_vectors(1, seed=3)[0]
        try:
            while not stop.is_set():
                store.search_vectors(query, 5)
        except BaseException as error:
            failures.append(error)

    thread = threading.Thread(target=searcher)
    thread.start()
    try:
        for batch in range(6):  # 6 x 500 chunks pushes past the initial 1024 rows: two growths
            chunks = [make_chunk(f"g{batch}-{i}") for i in range(500)]
            store.upsert(chunks, unit_vectors(500, seed=batch + 10))
    finally:
        stop.set()
        thread.join()
    assert failures == []
    assert store.count() == 3010


def test_random_operations_match_the_oracle_after_every_step(tmp_path):
    """Seeded mix of inserts, re-inserts, deletes and reopens, checked against brute force."""
    rng = np.random.default_rng(2024)
    directory = tmp_path / "idx"
    store = FlatStore(directory, model_id="fake", dim=DIM)
    model: dict[str, np.ndarray] = {}
    names = [f"n{i}" for i in range(120)]
    try:
        for step in range(80):
            op = str(rng.choice(["upsert", "reupsert", "delete", "reopen"], p=[0.4, 0.2, 0.3, 0.1]))
            if op in ("upsert", "reupsert"):
                pool = [n for n in names if (make_chunk(n).id in model) == (op == "reupsert")]
                if pool:
                    size = min(len(pool), int(rng.integers(1, 15)))
                    chunks = [make_chunk(str(n)) for n in rng.choice(pool, size, replace=False)]
                    vectors = unit_vectors(len(chunks), seed=int(rng.integers(1_000_000)))
                    store.upsert(chunks, vectors)
                    model.update({c.id: v for c, v in zip(chunks, vectors, strict=True)})
            elif op == "delete" and model:
                size = min(len(model), int(rng.integers(1, 10)))
                victims = [str(v) for v in rng.choice(sorted(model), size, replace=False)]
                assert store.delete(victims) == len(victims)
                for victim in victims:
                    del model[victim]
            elif op == "reopen":
                store.close()
                store = FlatStore(directory, model_id="fake", dim=DIM)

            assert store.count() == len(model), f"step {step} after {op}"
            for query in unit_vectors(3, seed=step):
                hits = store.search_vectors(query, 10)
                expected = oracle(model, query, 10)
                assert [h.id for h in hits] == [i for _, i in expected], f"step {step} after {op}"
                np.testing.assert_allclose(
                    [h.score for h in hits], [s for s, _ in expected], atol=1e-5
                )
    finally:
        store.close()
