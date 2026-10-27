import builtins
import io
from collections import Counter
from pathlib import Path

import pytest

from omni_memory.config import Settings
from omni_memory.embedding import FakeEmbedder
from omni_memory.indexer import Indexer
from omni_memory.store import FlatStore
from omni_memory.types import ChunkKind, IndexOptions

SAMPLE = Path(__file__).resolve().parents[2] / "Sample Data"
pytestmark = pytest.mark.skipif(not SAMPLE.is_dir(), reason="Sample Data/ is not present")


@pytest.fixture
def indexed(tmp_path, monkeypatch):
    embedder = FakeEmbedder(16)
    store = FlatStore(tmp_path / "idx", model_id=embedder.model_id, dim=16)
    opened: list[str] = []
    real_open = builtins.open

    def spy(file, *args, **kwargs):
        opened.append(str(file))
        return real_open(file, *args, **kwargs)

    monkeypatch.setattr(builtins, "open", spy)
    monkeypatch.setattr(io, "open", spy)
    indexer = Indexer(store, embedder, Settings(index_dir=tmp_path / "idx"))
    report = indexer.sync("sample", SAMPLE)
    yield store, indexer, report, opened
    store.close()


def test_the_real_sample_folder_is_indexed_and_secrets_are_not(indexed):
    store, _, report, opened = indexed
    uris = set(store.file_records("sample"))
    assert not [u for u in uris if ".env" in u or "node_modules" in u or ".git/" in u]
    assert not [name for name in opened if ".env" in Path(name).name]
    assert report.denied >= 4  # env files, lockfiles, .git, node_modules
    assert report.errors == []
    for expected in (
        "NextPlore/Backend/src/utils/jwt.js",
        "NextPlore/README.md",
        "NextPlore/Documentation/Images/dataschemas.png",
        "Week 10 Prep.pdf",
        "Week 11 Prep.pdf",
    ):
        assert expected in uris


def test_chunk_kinds_match_what_is_on_disk(indexed):
    store, _, _, _ = indexed
    ids = store._catalog.chunk_ids_for_source("sample")
    kinds = Counter(chunk.kind for chunk in store.get(ids).values())
    # `auto`: of the 9 + 11 pages, the 11 with a picture, a table or little text
    assert kinds[ChunkKind.PDF_PAGE] == 11
    assert kinds[ChunkKind.IMAGE] == 26  # 3 PNGs in the repo + 23 pictures at the top level
    assert kinds[ChunkKind.CODE] > 100
    assert kinds[ChunkKind.MARKDOWN] >= 3


def page_count(store):
    chunks = store.get(store._catalog.chunk_ids_for_source("sample")).values()
    return sum(chunk.kind is ChunkKind.PDF_PAGE for chunk in chunks)


def test_changing_the_pdf_option_processes_only_the_pdfs_again(indexed):
    store, indexer, _, _ = indexed
    every = indexer.sync("sample", SAMPLE, options=IndexOptions(pdf_pages="all"))
    assert page_count(store) == 20 and every.updated == 2
    words = indexer.sync("sample", SAMPLE, options=IndexOptions(pdf_pages="text"))
    assert page_count(store) == 0 and words.updated == 2


def test_syncing_again_does_no_work(indexed):
    store, indexer, first, _ = indexed
    count = store.count()
    second = indexer.sync("sample", SAMPLE)
    assert (second.added, second.updated) == (0, 0)
    assert second.unchanged == first.added
    assert store.count() == count
