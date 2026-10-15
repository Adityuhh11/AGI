import sqlite3

import numpy as np
import pytest

from agi_memory.store import FlatStore, IndexMismatchError
from helpers import DIM, file_record, make_file_chunks, unit_vectors


def put(store, uri, count, *, tag="v1", sha="sha1", source_id="s", mtime_ns=1):
    chunks = make_file_chunks(uri, count, source_id=source_id, tag=tag)
    store.replace_file(
        file_record(uri, source_id=source_id, sha=sha, mtime_ns=mtime_ns),
        chunks,
        unit_vectors(count),
    )
    return chunks


def test_a_new_source_has_no_files(store):
    store.register_source("s", "C:/data")
    assert store.file_records("s") == {}


def test_source_root_is_the_last_registered_folder(store):
    assert store.source_root("s") is None
    store.register_source("s", "C:/a")
    assert store.source_root("s") == "C:/a"


def test_register_source_is_idempotent(store):
    store.register_source("s", "C:/a")
    store.register_source("s", "C:/b")
    store.mark_scanned("s", 123)  # must not raise for a registered source


def test_replace_file_stores_chunks_and_the_file_record(store):
    chunks = put(store, "a.txt", 3)
    assert store.count() == 3
    assert set(store.get([c.id for c in chunks])) == {c.id for c in chunks}
    assert store.file_records("s") == {"a.txt": file_record("a.txt")}


def test_replacing_with_new_content_removes_the_stale_chunks(store):
    old = put(store, "a.txt", 5, tag="v1", sha="one")
    new = put(store, "a.txt", 2, tag="v2", sha="two", mtime_ns=9)
    assert store.count() == 2
    assert store.get([c.id for c in old]) == {}
    assert set(store.get([c.id for c in new])) == {c.id for c in new}
    assert store.file_records("s")["a.txt"].sha256 == "two"
    assert store.file_records("s")["a.txt"].mtime_ns == 9


def test_replacing_with_identical_content_changes_nothing(store):
    put(store, "a.txt", 3)
    put(store, "a.txt", 3)
    assert store.count() == 3


def test_a_file_that_becomes_empty_keeps_its_record_but_no_chunks(store):
    put(store, "a.txt", 3)
    store.replace_file(file_record("a.txt", sha="empty"), [], np.zeros((0, DIM), dtype=np.float32))
    assert store.count() == 0
    assert store.file_records("s")["a.txt"].sha256 == "empty"


def test_chunks_of_another_file_or_source_are_rejected_and_nothing_is_written(store):
    wrong_uri = make_file_chunks("b.txt", 1)
    wrong_source = make_file_chunks("a.txt", 1, source_id="other")
    for chunks in (wrong_uri, wrong_source):
        with pytest.raises(ValueError, match="belong"):
            store.replace_file(file_record("a.txt"), chunks, unit_vectors(1))
    assert store.count() == 0
    assert store.file_records("s") == {}


def test_touch_file_updates_only_the_record(store):
    chunks = put(store, "a.txt", 2)
    store.touch_file(file_record("a.txt", mtime_ns=99))
    assert store.file_records("s")["a.txt"].mtime_ns == 99
    assert set(store.get([c.id for c in chunks])) == {c.id for c in chunks}


def test_remove_files_deletes_chunks_and_records(store):
    put(store, "a.txt", 2)
    keep = put(store, "b.txt", 3)
    assert store.remove_files("s", ["a.txt", "never-existed.txt"]) == 1
    assert store.count() == 3
    assert set(store.file_records("s")) == {"b.txt"}
    assert set(store.get([c.id for c in keep])) == {c.id for c in keep}


def test_delete_source_only_touches_that_source(store):
    put(store, "a.txt", 2, source_id="one")
    keep = put(store, "a.txt", 2, source_id="two")
    store.register_source("one", "C:/one")
    assert store.delete_source("one") == 2
    assert store.file_records("one") == {}
    assert store.count() == 2
    assert set(store.get([c.id for c in keep])) == {c.id for c in keep}


def test_the_same_path_in_two_sources_does_not_collide(store):
    a = put(store, "same.txt", 1, source_id="one")
    b = put(store, "same.txt", 1, source_id="two")
    assert a[0].id != b[0].id
    assert store.count() == 2


def test_stale_chunks_are_not_searchable_after_replace(store):
    old = put(store, "a.txt", 4, tag="v1")
    vectors = unit_vectors(1, seed=3)
    new = make_file_chunks("a.txt", 1, tag="v2")
    store.replace_file(file_record("a.txt", sha="two"), new, vectors)
    hits = store.search_vectors(vectors[0], 10)
    assert [h.id for h in hits] == [new[0].id]
    assert not {h.id for h in hits} & {c.id for c in old}


def test_an_index_from_the_previous_schema_is_refused(tmp_path):
    directory = tmp_path / "idx"
    FlatStore(directory, model_id="fake", dim=DIM).close()
    db = sqlite3.connect(directory / "meta.db")
    db.execute("UPDATE meta SET value = '1' WHERE key = 'schema_version'")
    db.commit()
    db.close()
    with pytest.raises(IndexMismatchError, match="reindex"):
        FlatStore(directory, model_id="fake", dim=DIM)


def test_a_refused_index_is_left_exactly_as_it_was(tmp_path):
    directory = tmp_path / "idx"
    directory.mkdir()
    db = sqlite3.connect(directory / "meta.db")
    db.execute("CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)")
    db.executemany(
        "INSERT INTO meta VALUES (?, ?)",
        [("schema_version", "2"), ("model_id", "fake"), ("dim", str(DIM))],
    )
    db.commit()
    db.close()
    with pytest.raises(IndexMismatchError, match="reindex"):
        FlatStore(directory, model_id="fake", dim=DIM)
    db = sqlite3.connect(directory / "meta.db")
    tables = {name for (name,) in db.execute("SELECT name FROM sqlite_master")}
    db.close()
    assert tables == {"meta", "sqlite_autoindex_meta_1"}  # no new tables were created in it
