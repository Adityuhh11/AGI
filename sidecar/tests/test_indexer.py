import builtins
import io
import os

import pytest
from PIL import Image

from agi_memory.config import Settings
from agi_memory.embedding import FakeEmbedder
from agi_memory.indexer import Indexer
from agi_memory.store import FlatStore
from agi_memory.types import ChunkKind
from pdf_fixture import make_text_pdf

DIM = 16


class CountingEmbedder(FakeEmbedder):
    def __init__(self) -> None:
        super().__init__(DIM)
        self.document_calls = 0
        self.image_calls = 0

    def embed_documents(self, texts):
        self.document_calls += 1
        return super().embed_documents(texts)

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


def test_searching_finds_what_was_indexed(indexer, store, embedder, root):
    write(root, "a.js", "function frobnicate() {}")
    indexer.sync("s", root)
    (hit,) = store.search_vectors(embedder.embed_query("frobnicate"), 1)
    assert store.get([hit.id])[hit.id].uri == "a.js"


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
        own_vector = embedder.embed_documents([chunk.text])[0]
        assert store.search_vectors(own_vector, 1)[0].id == chunk.id
