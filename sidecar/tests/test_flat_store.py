import shutil
import sqlite3

import numpy as np
import pytest

from agi_memory.store import FlatStore, IndexMismatchError
from helpers import DIM, make_chunk, unit_vectors


def fts_ids(directory, term):
    """Ids the keyword index finds for `term`, read through a separate connection."""
    db = sqlite3.connect(directory / "meta.db")
    try:
        sql = (
            "SELECT c.id FROM chunks_fts f JOIN chunks c ON c.rid = f.rowid "
            "WHERE chunks_fts MATCH ?"
        )
        return {row[0] for row in db.execute(sql, (term,))}
    finally:
        db.close()


def test_upsert_then_get_roundtrip(store):
    a, b = make_chunk("a"), make_chunk("b", "other text")
    store.upsert([a, b], unit_vectors(2))
    assert store.get([a.id, b.id]) == {a.id: a, b.id: b}
    assert store.count() == 2


def test_new_store_is_empty(store):
    assert store.count() == 0
    assert store.get(["nope"]) == {}


def test_upserting_an_existing_id_updates_in_place(store):
    store.upsert([make_chunk("a", mtime_ns=1)], unit_vectors(1))
    newer = make_chunk("a", mtime_ns=2)
    store.upsert([newer], unit_vectors(1, seed=1))
    assert store.count() == 1
    assert store.get([newer.id])[newer.id].mtime_ns == 2


def test_duplicate_ids_in_one_batch_are_rejected(store):
    with pytest.raises(ValueError, match="duplicate"):
        store.upsert([make_chunk("a"), make_chunk("a")], unit_vectors(2))
    assert store.count() == 0


def test_empty_batch_is_a_noop(store):
    store.upsert([], np.zeros((0, DIM), dtype=np.float32))
    assert store.count() == 0


def bad_vectors():
    good = unit_vectors(1)
    nan = good.copy()
    nan[0, 0] = np.nan
    return {
        "wrong dimension": unit_vectors(1, dim=DIM + 1),
        "float64": good.astype(np.float64),
        "nan": nan,
        "not unit length": good * 2,
        "wrong row count": unit_vectors(2),
    }


@pytest.mark.parametrize("label", list(bad_vectors()))
def test_bad_vectors_are_rejected_before_touching_disk(store, label):
    with pytest.raises(ValueError):
        store.upsert([make_chunk("a")], bad_vectors()[label])
    assert store.count() == 0


def test_delete_removes_and_reports_how_many_existed(store):
    a, b = make_chunk("a"), make_chunk("b")
    store.upsert([a, b], unit_vectors(2))
    assert store.delete([a.id, "missing"]) == 1
    assert store.get([a.id, b.id]) == {b.id: b}
    assert store.count() == 1


def test_everything_survives_reopen(tmp_path):
    directory = tmp_path / "idx"
    first = FlatStore(directory, model_id="fake", dim=DIM)
    chunks = [make_chunk("a"), make_chunk("b")]
    first.upsert(chunks, unit_vectors(2))
    first.close()

    second = FlatStore(directory, model_id="fake", dim=DIM)
    assert second.count() == 2
    assert set(second.get([c.id for c in chunks])) == {c.id for c in chunks}
    second.close()


def test_reopening_with_another_model_or_dimension_is_refused(tmp_path):
    directory = tmp_path / "idx"
    FlatStore(directory, model_id="m1", dim=DIM).close()
    with pytest.raises(IndexMismatchError, match="reindex"):
        FlatStore(directory, model_id="m2", dim=DIM)
    with pytest.raises(IndexMismatchError, match="reindex"):
        FlatStore(directory, model_id="m1", dim=DIM + 1)


def test_keyword_index_follows_writes(tmp_path):
    directory = tmp_path / "idx"
    store = FlatStore(directory, model_id="fake", dim=DIM)
    original = make_chunk("a", "addCountries zebra")
    store.upsert([original], unit_vectors(1))
    assert fts_ids(directory, "zebra") == {original.id}
    assert fts_ids(directory, "countries") == {original.id}  # via split identifiers
    assert fts_ids(directory, "addcountries") == {original.id}  # via the original token

    replaced = make_chunk("a", "now says giraffe", mtime_ns=2)
    store.upsert([replaced], unit_vectors(1))
    assert fts_ids(directory, "zebra") == set()
    assert fts_ids(directory, "giraffe") == {replaced.id}

    store.delete([replaced.id])
    assert fts_ids(directory, "giraffe") == set()
    store.close()


def test_hostile_text_is_stored_verbatim(store):
    nasty = "say \"hi\" * NEAR( x ) 'quoted' 🙂 line1\r\nline2"
    chunk = make_chunk("a", nasty)
    store.upsert([chunk], unit_vectors(1))
    assert store.get([chunk.id])[chunk.id].text == nasty


def test_closed_store_leaves_no_open_handles(tmp_path):
    directory = tmp_path / "idx"
    store = FlatStore(directory, model_id="fake", dim=DIM)
    store.upsert([make_chunk("a")], unit_vectors(1))
    store.close()
    shutil.rmtree(directory)  # raises PermissionError on Windows if a handle leaked


@pytest.mark.parametrize("method", ["delete", "get"])
def test_a_bare_string_is_not_a_list_of_ids(store, method):
    chunk = make_chunk("a")
    store.upsert([chunk], unit_vectors(1))
    with pytest.raises(TypeError, match="single string"):
        getattr(store, method)(chunk.id)
    assert store.count() == 1


def test_warming_an_index_changes_nothing_a_search_can_see(store):
    store.warm()  # an empty index
    a, b = make_chunk("a"), make_chunk("b", "other text")
    store.upsert([a, b], unit_vectors(2))
    store.warm()
    assert store.search_vectors(unit_vectors(2)[0], 2)[0].id == a.id
    assert store.count() == 2
