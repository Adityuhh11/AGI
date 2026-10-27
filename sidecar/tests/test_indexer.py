import builtins
import io
import os
from collections import Counter
from pathlib import Path

import numpy as np
import pytest
from PIL import Image

from omni_memory.config import Settings
from omni_memory.embedding import FakeEmbedder
from omni_memory.indexer import Indexer
from omni_memory.store import FlatStore
from omni_memory.types import LEGACY_OPTIONS, ChunkKind, IndexOptions
from pdf_fixture import make_text_pdf
from video_fixture import make_video

DIM = 16


class CountingEmbedder(FakeEmbedder):
    def __init__(self) -> None:
        super().__init__(DIM)
        self.document_calls = 0
        self.titles: list[str] = []
        self.image_calls = 0

    def embed_documents(self, texts, titles=None):
        self.document_calls += 1
        self.titles.extend(titles or [])
        return super().embed_documents(texts, titles)

    def embed_images(self, images):
        self.image_calls += 1
        return super().embed_images(images)


@pytest.fixture
def embedder():
    return CountingEmbedder()


@pytest.fixture
def settings(tmp_path):
    return Settings(index_dir=tmp_path / "idx")


@pytest.fixture
def store(tmp_path):
    flat = FlatStore(tmp_path / "idx", model_id="fake", dim=DIM)
    yield flat
    flat.close()


@pytest.fixture
def indexer(store, embedder, settings):
    return Indexer(store, embedder, settings, clock=lambda: 42)


@pytest.fixture
def root(tmp_path):
    folder = tmp_path / "src"
    folder.mkdir()
    return folder


def write(root, relative, content="x"):
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return path


def all_chunks(store, source_id="s"):
    return store.get(store._catalog.chunk_ids_for_source(source_id))


def test_first_sync_indexes_everything_and_reports_it(indexer, store, root):
    write(root, "a.js", "const alpha = 1")
    write(root, "docs/b.md", "# Title\nbeta text")
    report = indexer.sync("s", root)
    assert (report.added, report.updated, report.unchanged, report.removed) == (2, 0, 0, 0)
    assert report.chunks_written == store.count() == 2
    assert set(store.file_records("s")) == {"a.js", "docs/b.md"}


def test_text_chunks_are_embedded_with_their_file_name_as_title(indexer, embedder, root):
    write(root, "src/deep/Widget.js", "function render() {}")
    write(root, "notes.md", "# Notes\nhello")
    indexer.sync("s", root)
    assert sorted(set(embedder.titles)) == ["Widget.js", "notes.md"]


def test_searching_finds_what_was_indexed(indexer, store, embedder, root):
    write(root, "a.js", "function frobnicate() {}")
    indexer.sync("s", root)
    (hit,) = store.search_vectors(embedder.embed_query("frobnicate"), 1)
    assert store.get([hit.id])[hit.id].uri == "a.js"


def test_the_newest_files_are_indexed_first(indexer, root, monkeypatch):
    for name, age in (("old.txt", 300), ("newest.txt", 0), ("middle.txt", 100)):
        path = write(root, name, name)
        stamp = path.stat().st_mtime - age
        os.utime(path, (stamp, stamp))
    seen = []
    original = Indexer._index_file

    def spy(self, run, file, known):
        seen.append(file.uri)
        return original(self, run, file, known)

    monkeypatch.setattr(Indexer, "_index_file", spy)
    indexer.sync("s", root)
    assert seen == ["newest.txt", "middle.txt", "old.txt"]


def test_an_unchanged_folder_costs_no_embedding_work(indexer, store, embedder, root):
    write(root, "a.js", "one")
    write(root, "b.md", "two")
    indexer.sync("s", root)
    before = {i: c for i, c in all_chunks(store).items()}
    calls = embedder.document_calls
    report = indexer.sync("s", root)
    assert (report.added, report.updated, report.unchanged) == (0, 0, 2)
    assert embedder.document_calls == calls
    assert all_chunks(store) == before


def test_a_modified_file_is_reindexed_and_stale_chunks_disappear(indexer, store, root):
    path = write(root, "a.txt", "\n".join(f"line {i}" for i in range(100)))
    indexer.sync("s", root)
    assert store.count() == 3
    path.write_text("short now", encoding="utf-8")
    report = indexer.sync("s", root)
    assert (report.updated, report.added) == (1, 0)
    assert store.count() == 1
    assert "short now" in next(iter(all_chunks(store).values())).text


def test_an_edit_embeds_only_the_chunks_it_changed(indexer, store, embedder, root):
    lines = [f"line {n} of the notes" for n in range(100)]
    write(root, "notes.txt", "\n".join(lines))
    indexer.sync("s", root)
    before = len(embedder.titles)
    write(root, "notes.txt", "\n".join([*lines, "one more line at the end"]))
    indexer.sync("s", root)
    assert len(embedder.titles) - before == 1  # three windows are unchanged, one is new
    assert len(all_chunks(store)) == 4


def test_rewriting_identical_bytes_only_updates_the_record(indexer, store, embedder, root):
    path = write(root, "a.txt", "same content")
    indexer.sync("s", root)
    calls = embedder.document_calls
    os.utime(path, ns=(10**18, 10**18))  # new mtime, same bytes
    report = indexer.sync("s", root)
    assert (report.unchanged, report.updated) == (1, 0)
    assert embedder.document_calls == calls
    assert store.file_records("s")["a.txt"].mtime_ns == 10**18


def test_deleted_files_are_removed_from_the_index(indexer, store, root):
    write(root, "keep.txt", "keep")
    gone = write(root, "gone.txt", "gone")
    indexer.sync("s", root)
    gone.unlink()
    report = indexer.sync("s", root)
    assert report.removed == 1
    assert set(store.file_records("s")) == {"keep.txt"}
    assert store.count() == 1


def test_secret_files_are_never_opened_or_indexed(indexer, store, root, monkeypatch):
    write(root, ".env", "API_KEY=TOPSECRET-123")
    write(root, "Backend/.env.local", "DB_PASSWORD=TOPSECRET-456")
    write(root, "keys/server.pem", "-----BEGIN PRIVATE KEY-----")
    write(root, "ok.js", "const fine = true")
    opened: list[str] = []
    real_open = builtins.open

    def spy(file, *args, **kwargs):
        opened.append(str(file))
        return real_open(file, *args, **kwargs)

    monkeypatch.setattr(builtins, "open", spy)
    monkeypatch.setattr(io, "open", spy)
    report = indexer.sync("s", root)
    assert report.denied == 3
    assert not [name for name in opened if name.endswith((".env", ".env.local", ".pem"))]
    assert all("TOPSECRET" not in c.text for c in all_chunks(store).values())
    assert set(store.file_records("s")) == {"ok.js"}


def test_binary_files_with_code_extensions_are_skipped_not_errors(indexer, root):
    (root / "weird.js").write_bytes(b"\x00\x01\x02 not really javascript")
    write(root, "fine.js", "ok")
    report = indexer.sync("s", root)
    assert (report.unsupported, report.added, report.errors) == (1, 1, [])


def test_a_file_that_cannot_be_read_is_an_error_and_the_rest_is_indexed(
    indexer, store, root, monkeypatch
):
    write(root, "locked.txt", "cannot read me")
    write(root, "fine.txt", "readable")
    real = type(root).read_bytes

    def flaky(self):
        if self.name == "locked.txt":
            raise PermissionError("file is locked")
        return real(self)

    monkeypatch.setattr(type(root), "read_bytes", flaky)
    report = indexer.sync("s", root)
    assert [(e.uri, "locked" in e.message) for e in report.errors] == [("locked.txt", True)]
    assert report.added == 1
    assert set(store.file_records("s")) == {"fine.txt"}


def test_a_corrupt_pdf_and_a_bad_image_are_recorded_and_others_survive(indexer, root):
    (root / "broken.pdf").write_bytes(b"%PDF-1.4 not really")
    (root / "broken.png").write_bytes(b"not an image")
    write(root, "fine.txt", "readable")
    report = indexer.sync("s", root)
    assert sorted(e.uri for e in report.errors) == ["broken.pdf", "broken.png"]
    assert report.added == 1


def test_a_text_pdf_yields_text_and_page_image_chunks(indexer, store, root):
    (root / "paper.pdf").write_bytes(make_text_pdf(["Confidence intervals", "z table"]))
    report = indexer.sync("s", root)
    kinds = sorted(c.kind for c in all_chunks(store).values())
    assert kinds == [ChunkKind.PDF_PAGE] * 2 + [ChunkKind.PDF_TEXT] * 2
    assert (report.added, report.chunks_written) == (1, 4)


def test_an_image_file_is_one_image_chunk(indexer, store, root):
    buffer = io.BytesIO()
    Image.new("RGB", (64, 48), "red").save(buffer, format="PNG")
    (root / "pics").mkdir()
    (root / "pics" / "red square.png").write_bytes(buffer.getvalue())
    indexer.sync("s", root)
    (chunk,) = all_chunks(store).values()
    assert (chunk.kind, chunk.uri, chunk.text) == (
        ChunkKind.IMAGE, "pics/red square.png", "pics/red square.png",
    )  # fmt: skip


def test_text_chunks_are_embedded_in_batches_across_files(store, embedder, settings, root):
    for number in range(30):
        write(root, f"f{number}.txt", f"file number {number}")
    batched = Indexer(store, embedder, Settings(index_dir=settings.index_dir, flush_chunks=10))
    batched.sync("s", root)
    assert store.count() == 30
    assert embedder.document_calls <= 4  # 30 chunks in batches of ~10, not one call per file


def test_an_unreadable_directory_never_deletes_its_indexed_files(indexer, store, root, monkeypatch):
    write(root, "ok.txt", "fine")
    write(root, "locked/hidden.txt", "indexed earlier")
    indexer.sync("s", root)
    real_scandir = os.scandir

    def flaky(path):
        if str(path).endswith("locked"):
            raise PermissionError("access denied")
        return real_scandir(path)

    monkeypatch.setattr(os, "scandir", flaky)
    report = indexer.sync("s", root)
    assert report.removed == 0
    assert set(store.file_records("s")) == {"ok.txt", "locked/hidden.txt"}
    assert [e.uri for e in report.errors] == ["locked/"]


def test_a_missing_source_root_is_an_error(indexer, tmp_path):
    with pytest.raises(FileNotFoundError, match="source root"):
        indexer.sync("s", tmp_path / "unplugged-drive")


def test_two_sources_with_identical_files_stay_separate(indexer, store, root, tmp_path):
    second = tmp_path / "other"
    second.mkdir()
    write(root, "same.txt", "identical")
    write(second, "same.txt", "identical")
    indexer.sync("one", root)
    indexer.sync("two", second)
    assert store.count() == 2
    assert indexer.sync("one", root).unchanged == 1


def test_progress_is_reported_per_file_and_scan_time_is_recorded(indexer, store, root):
    write(root, "a.txt", "a")
    write(root, "b.txt", "b")
    seen = []
    indexer.sync("s", root, on_progress=lambda done, total: seen.append((done, total)))
    assert seen == [(1, 2), (2, 2)]
    row = store._db.execute("SELECT last_scan_ns FROM sources WHERE id = 's'").fetchone()
    assert row == (42,)


def test_empty_and_hostile_files_do_not_break_a_sync(indexer, store, root):
    write(root, "empty.txt", "")
    (root / "invalid.txt").write_bytes(b"ok \xff\xfe bad")
    write(root, "huge-line.js", "x" * 1_000_000)
    write(root, "crlf.md", "# H\r\nbody\r\n")
    report = indexer.sync("s", root)
    assert report.errors == []
    assert report.added == 4  # the empty file is "indexed" with zero chunks
    assert store.count() == 3


def test_a_malformed_image_does_not_abort_the_sync(indexer, store, root):
    from test_chunk_image import MALFORMED

    write(root, "a.txt", "readable before")
    (root / "b.png").write_bytes(bytes.fromhex(MALFORMED[0]))
    write(root, "c.txt", "readable after")
    report = indexer.sync("s", root)
    assert [e.uri for e in report.errors] == ["b.png"]
    assert set(store.file_records("s")) == {"a.txt", "c.txt"}


def test_an_interrupted_update_never_leaves_stale_chunks_searchable_after_a_revert(
    indexer, store, embedder, root, monkeypatch
):
    path = write(root, "a.txt", "original apple text")
    indexer.sync("s", root)
    path.write_text("edited banana text!!", encoding="utf-8")

    def crash(ids):
        raise RuntimeError("database is locked")

    with monkeypatch.context() as patch:
        patch.setattr(store, "delete", crash)
        with pytest.raises(RuntimeError):
            indexer.sync("s", root)
    path.write_text("original apple text", encoding="utf-8")  # reverted before the next sync
    indexer.sync("s", root)
    assert store.keyword_search("banana", 5) == []
    assert store.count() == 1


def test_every_file_gets_its_own_vectors_across_batched_flushes(store, embedder, settings, root):
    for number in range(25):
        write(root, f"f{number}.txt", f"unique{number} content {number * 7}")
    Indexer(store, embedder, Settings(index_dir=settings.index_dir, flush_chunks=8)).sync("s", root)
    chunks = all_chunks(store)
    assert len(chunks) == 25
    for chunk in chunks.values():
        own_vector = embedder.embed_documents([chunk.text], [Path(chunk.uri).name])[0]
        assert store.search_vectors(own_vector, 1)[0].id == chunk.id


def png(colour):
    buffer = io.BytesIO()
    Image.new("RGB", (8, 8), colour).save(buffer, format="PNG")
    return buffer.getvalue()


def vector_of(store, uri):
    (vector,) = store.vectors_of("s", uri, [ChunkKind.IMAGE]).values()
    return vector


def test_a_moved_image_keeps_its_vector_and_costs_no_embedding(indexer, store, embedder, root):
    (root / "inbox").mkdir()
    (root / "inbox" / "photo.png").write_bytes(png("red"))
    indexer.sync("s", root)
    before = vector_of(store, "inbox/photo.png")
    calls = embedder.image_calls

    (root / "albums").mkdir()
    (root / "inbox" / "photo.png").rename(root / "albums" / "holiday.png")  # moved and renamed
    report = indexer.sync("s", root)

    assert embedder.image_calls == calls
    assert (report.added, report.removed, report.reused) == (1, 1, 1)
    assert sorted(c.uri for c in all_chunks(store).values()) == ["albums/holiday.png"]
    after = vector_of(store, "albums/holiday.png")
    assert (after == before).all()


def test_a_copy_of_an_indexed_image_reuses_its_vector(indexer, store, embedder, root):
    (root / "a.png").write_bytes(png("green"))
    indexer.sync("s", root)
    calls = embedder.image_calls
    (root / "b.png").write_bytes(png("green"))
    report = indexer.sync("s", root)
    assert embedder.image_calls == calls
    assert report.reused == 1
    assert (vector_of(store, "a.png") == vector_of(store, "b.png")).all()


def test_a_moved_pdf_reuses_its_page_images_but_embeds_its_text_again(
    indexer, store, embedder, root
):
    (root / "paper.pdf").write_bytes(make_text_pdf(["Confidence intervals", "z table"]))
    indexer.sync("s", root)
    pages = store.vectors_of("s", "paper.pdf", [ChunkKind.PDF_PAGE])
    images, documents = embedder.image_calls, embedder.document_calls

    (root / "paper.pdf").rename(root / "stats.pdf")
    report = indexer.sync("s", root)

    assert embedder.image_calls == images  # no page was rendered or embedded
    assert embedder.document_calls > documents  # the text carries the path, so it is embedded again
    assert report.reused == 1
    moved = store.vectors_of("s", "stats.pdf", [ChunkKind.PDF_PAGE])
    assert moved.keys() == pages.keys()
    assert all((moved[k] == pages[k]).all() for k in pages)
    assert {c.uri for c in all_chunks(store).values()} == {"stats.pdf"}


def test_a_file_edited_in_the_same_sync_is_never_a_donor(indexer, store, embedder, root):
    (root / "a.png").write_bytes(png("red"))
    indexer.sync("s", root)
    old_red = vector_of(store, "a.png")

    # a.png now holds blue, and b.png appears with a.png's old red bytes: b must get red's vector,
    # not the blue one a.png is about to receive.
    (root / "a.png").write_bytes(png("blue"))
    (root / "b.png").write_bytes(png("red"))
    report = indexer.sync("s", root)

    assert report.reused == 0
    assert (vector_of(store, "b.png") == old_red).all()
    assert not (vector_of(store, "a.png") == old_red).all()


def test_different_images_never_share_a_vector(indexer, store, root):
    (root / "r.png").write_bytes(png("red"))
    (root / "g.png").write_bytes(png("green"))
    report = indexer.sync("s", root)
    assert report.reused == 0
    assert not (vector_of(store, "r.png") == vector_of(store, "g.png")).all()


def test_excluding_a_folder_drops_what_was_indexed_from_it(indexer, store, root):
    write(root, "keep.txt", "keep this")
    write(root, "vendor/skills/a.txt", "vendored")
    write(root, "vendor/skills/deep/b.txt", "vendored too")
    write(root, "vendor/mine.txt", "my own")
    assert indexer.sync("s", root).added == 4
    report = indexer.sync("s", root, exclude=["vendor/skills/"])
    assert (report.removed, report.ignored, report.added) == (2, 1, 0)
    assert set(store.file_records("s")) == {"keep.txt", "vendor/mine.txt"}


def test_any_failure_in_one_file_skips_that_file_and_not_the_folder(
    indexer, store, root, monkeypatch
):
    write(root, "bad.txt", "this one makes a library blow up")
    write(root, "fine.txt", "readable")
    import omni_memory.indexer as module

    real = module.chunk_text_document

    def exploding(source_id, uri, *args, **kwargs):
        if uri == "bad.txt":
            raise RuntimeError("a library failed")
        return real(source_id, uri, *args, **kwargs)

    monkeypatch.setattr(module, "chunk_text_document", exploding)
    report = indexer.sync("s", root)
    assert [(e.uri, e.message) for e in report.errors] == [
        ("bad.txt", "RuntimeError: a library failed")
    ]
    assert set(store.file_records("s")) == {"fine.txt"}


def test_a_run_of_failures_aborts_the_sync_because_the_files_are_not_the_cause(
    indexer, root, monkeypatch
):
    for number in range(8):
        write(root, f"f{number}.txt", "x")
    import omni_memory.indexer as module

    def broken(*args, **kwargs):
        raise RuntimeError("the GPU is gone")

    monkeypatch.setattr(module, "chunk_text_document", broken)
    with pytest.raises(RuntimeError, match="the GPU is gone"):
        indexer.sync("s", root)


def test_any_number_of_corrupt_files_in_a_row_never_aborts_the_sync(indexer, store, root):
    for number in range(8):
        (root / f"a{number}.png").write_bytes(b"not an image")
    write(root, "z.txt", "fine")
    report = indexer.sync("s", root)
    assert (len(report.errors), report.added) == (8, 1)


# --- index options ------------------------------------------------------------------------


def write_png(path, colour="red"):
    path.parent.mkdir(parents=True, exist_ok=True)
    buffer = io.BytesIO()
    Image.new("RGB", (64, 48), colour).save(buffer, format="PNG")
    path.write_bytes(buffer.getvalue())


def test_photos_by_name_are_embedded_from_their_name_not_their_pixels(
    indexer, store, embedder, root
):
    write_png(root / "trips" / "beach.png")
    indexer.sync("s", root, options=IndexOptions(photos="name"))
    (chunk,) = all_chunks(store).values()
    assert (chunk.kind, chunk.text) == (ChunkKind.IMAGE, "trips/beach.png")
    assert embedder.image_calls == 0 and embedder.titles == ["beach.png"]


def test_changing_an_option_processes_that_kind_again_and_nothing_else(
    indexer, store, embedder, root
):
    write_png(root / "beach.png")
    write(root, "notes.md", "# Notes\nsome words")
    indexer.sync("s", root, options=IndexOptions(photos="name"))
    again = indexer.sync("s", root, options=IndexOptions(photos="name"))
    assert (again.updated, again.unchanged) == (0, 2)  # the same options: nothing to do
    switched = indexer.sync("s", root, options=IndexOptions(photos="content"))
    assert (switched.updated, switched.unchanged) == (1, 1)  # the picture, not the notes
    assert embedder.image_calls == 1
    back = indexer.sync("s", root, options=IndexOptions(photos="name"))
    assert back.updated == 1 and embedder.image_calls == 1
    # the notes once; the name twice, because the switch to pixels removed its chunk
    assert sorted(embedder.titles) == ["beach.png", "beach.png", "notes.md"]


class RefusingEmbedder(CountingEmbedder):
    """Plays a server that refuses some pictures: those `refuse` says no to, or all of them."""

    def __init__(self, refuse) -> None:
        super().__init__()
        self.refuse = refuse

    def embed_images(self, images):
        if any(self.refuse(image) for image in images):
            raise RuntimeError("llama-server answered 500")
        return super().embed_images(images)


def test_a_picture_the_server_refuses_is_that_files_error_and_the_rest_are_indexed(
    store, settings, root
):
    for number, width in enumerate([64, 13, 64, 64]):  # a batch of four, one of them refused
        path = root / f"p{number}.png"
        Image.new("RGB", (width, 48), (number * 50, 0, 0)).save(path)
    refusing = RefusingEmbedder(lambda image: image.size[0] == 13)
    report = Indexer(store, refusing, settings).sync("s", root)
    assert [e.uri for e in report.errors] == ["p1.png"]
    assert "answered 500" in report.errors[0].message
    assert sorted(store.file_records("s")) == ["p0.png", "p2.png", "p3.png"]


def test_when_every_picture_of_a_batch_fails_the_sync_stops(store, settings, root):
    for number in range(4):
        Image.new("RGB", (64, 48), (number * 50, 0, 0)).save(root / f"p{number}.png")
    with pytest.raises(RuntimeError, match="answered 500"):
        Indexer(store, RefusingEmbedder(lambda image: True), settings).sync("s", root)


def test_a_photo_moved_while_switching_to_content_never_inherits_its_name_vector(
    indexer, store, embedder, root
):
    write_png(root / "inbox" / "photo.png")
    indexer.sync("s", root, options=IndexOptions(photos="name"))
    by_name = vector_of(store, "inbox/photo.png")
    (root / "inbox" / "photo.png").rename(root / "kept.png")
    report = indexer.sync("s", root, options=IndexOptions(photos="content"))
    assert embedder.image_calls == 1 and report.reused == 0  # embedded from its pixels
    assert not np.allclose(vector_of(store, "kept.png"), by_name)


def test_a_finished_sync_leaves_its_options_and_nothing_to_redo(indexer, store, root):
    write(root, "a.txt", "x")
    assert store.source_options("s") is None
    indexer.sync("s", root, options=IndexOptions(pdf_pages="text"))
    assert store.source_options("s") == (IndexOptions(pdf_pages="text"), frozenset())


def test_a_change_cut_short_and_then_undone_still_puts_every_file_back(
    indexer, store, root, monkeypatch
):
    for name in ("a", "b"):
        write_png(root / f"{name}.png")
    indexer.sync("s", root)  # photos by what they show
    real = store.remove_files

    def cut_short(*args, **kwargs):  # every photo is converted, then the sync stops
        raise RuntimeError("the computer went to sleep")

    monkeypatch.setattr(store, "remove_files", cut_short)
    with pytest.raises(RuntimeError):
        indexer.sync("s", root, options=IndexOptions(photos="name"))
    assert store.source_options("s") == (IndexOptions(photos="name"), {"image"})
    monkeypatch.setattr(store, "remove_files", real)
    report = indexer.sync("s", root, options=IndexOptions(photos="content"))  # changed back
    assert report.updated == 2
    hashes = {c.content_hash for c in all_chunks(store).values()}
    assert not any(h.startswith("name:") for h in hashes)  # both by their pixels again
    assert store.source_options("s") == (IndexOptions(), frozenset())


def test_an_index_from_before_the_options_drops_plain_pages_without_embedding_anything(
    indexer, store, embedder, root
):
    prose = "plain words of a page of prose " * 10
    (root / "paper.pdf").write_bytes(make_text_pdf([prose, "Figure 1"]))
    indexer.sync("s", root, options=LEGACY_OPTIONS)
    store._db.execute("DELETE FROM source_options")  # as an index built before them
    images, documents = embedder.image_calls, embedder.document_calls
    report = indexer.sync("s", root)  # the default, auto
    pages = [c for c in all_chunks(store).values() if c.kind is ChunkKind.PDF_PAGE]
    assert [p.locator for p in pages] == ['{"page":2}'] and report.updated == 1
    assert (embedder.image_calls, embedder.document_calls) == (images, documents)


def test_a_video_is_found_by_its_name_by_default(indexer, store, embedder, root):
    make_video(root / "holiday.mp4")
    indexer.sync("s", root)
    (chunk,) = all_chunks(store).values()
    assert (chunk.kind, chunk.text) == (ChunkKind.VIDEO, "holiday.mp4")
    assert embedder.image_calls == 0


def test_a_video_by_content_has_its_name_and_eight_frames(indexer, store, embedder, root):
    make_video(root / "holiday.mp4")
    indexer.sync("s", root, options=IndexOptions(videos="content"))
    kinds = Counter(c.kind for c in all_chunks(store).values())
    assert kinds == {ChunkKind.VIDEO: 1, ChunkKind.VIDEO_FRAME: 8}
    calls = embedder.image_calls
    report = indexer.sync("s", root, options=IndexOptions(videos="name"))
    assert report.updated == 1
    assert Counter(c.kind for c in all_chunks(store).values()) == {ChunkKind.VIDEO: 1}
    assert embedder.image_calls == calls


def test_a_video_whose_frames_cannot_be_read_keeps_its_name(indexer, store, root):
    (root / "broken.mp4").write_bytes(b"not a video" * 100)
    report = indexer.sync("s", root, options=IndexOptions(videos="content"))
    (chunk,) = all_chunks(store).values()
    assert chunk.kind is ChunkKind.VIDEO
    assert [e.uri for e in report.errors] == ["broken.mp4"]
    assert "found by its name only" in report.errors[0].message
