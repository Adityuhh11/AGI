import shutil

import pytest

from helpers import DIM, make_chunk, unit_vectors
from omni_memory.store import FlatStore, IndexMismatchError
from omni_memory.store.vectors import VectorFile
from omni_memory.types import Filters


def test_failed_growth_leaves_the_store_usable(tmp_path, monkeypatch):
    directory = tmp_path / "idx"
    store = FlatStore(directory, model_id="fake", dim=DIM)
    kept = make_chunk("kept")
    store.upsert([kept], unit_vectors(1))

    def disk_full(self):
        raise OSError("disk full")

    big = [make_chunk(f"b{i}") for i in range(1100)]  # needs more than the initial 1024 rows
    with monkeypatch.context() as patch:
        patch.setattr(VectorFile, "_write_header", disk_full)
        with pytest.raises(OSError, match="disk full"):
            store.upsert(big, unit_vectors(1100, seed=2))

    assert store.count() == 1
    query = unit_vectors(1, seed=5)[0]
    assert [h.id for h in store.search_vectors(query, 10)] == [kept.id]
    assert [h.id for h in store.search_vectors(query, 10, Filters(source_id="s"))] == [kept.id]
    store.upsert(big[:5], unit_vectors(5, seed=3))  # still writable
    assert store.count() == 6
    store.close()
    shutil.rmtree(directory)  # no handle leaked on the failure path


def test_missing_vector_file_is_refused_when_chunks_exist(tmp_path):
    directory = tmp_path / "idx"
    store = FlatStore(directory, model_id="fake", dim=DIM)
    store.upsert([make_chunk("a")], unit_vectors(1))
    store.close()
    (directory / "vectors.f32").unlink()
    with pytest.raises(IndexMismatchError, match="vector file"):
        FlatStore(directory, model_id="fake", dim=DIM)
    shutil.rmtree(directory)  # the failed open leaked nothing


def test_unusable_vector_file_is_refused_when_chunks_exist(tmp_path):
    directory = tmp_path / "idx"
    store = FlatStore(directory, model_id="fake", dim=DIM)
    store.upsert([make_chunk("a")], unit_vectors(1))
    store.close()
    (directory / "vectors.f32").write_bytes(b"garbage" * 40)
    with pytest.raises(IndexMismatchError, match="reindex"):
        FlatStore(directory, model_id="fake", dim=DIM)
    shutil.rmtree(directory)


def test_chunk_pointing_past_the_vector_file_is_refused(tmp_path):
    directory = tmp_path / "idx"
    store = FlatStore(directory, model_id="fake", dim=DIM)
    store.upsert([make_chunk(f"c{i}") for i in range(3)], unit_vectors(3))
    store.close()
    path = directory / "vectors.f32"
    path.unlink()
    VectorFile(path, DIM, initial_capacity=1).close()  # a valid file, but only one row
    with pytest.raises(IndexMismatchError, match="reindex"):
        FlatStore(directory, model_id="fake", dim=DIM)
    shutil.rmtree(directory)


@pytest.mark.parametrize("damage", ["missing", "empty", "garbage"])
def test_empty_index_recreates_a_missing_or_broken_vector_file(tmp_path, damage):
    directory = tmp_path / "idx"
    FlatStore(directory, model_id="fake", dim=DIM).close()
    path = directory / "vectors.f32"
    if damage == "missing":
        path.unlink()
    elif damage == "empty":
        path.write_bytes(b"")  # a crash between creating the file and writing its header
    else:
        path.write_bytes(b"garbage" * 40)

    store = FlatStore(directory, model_id="fake", dim=DIM)
    chunk = make_chunk("a")
    vectors = unit_vectors(1)
    store.upsert([chunk], vectors)
    assert [h.id for h in store.search_vectors(vectors[0], 5)] == [chunk.id]
    store.close()
