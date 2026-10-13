import builtins
import io
from collections import Counter
from pathlib import Path

import pytest

from agi_memory.config import Settings
from agi_memory.embedding import FakeEmbedder
from agi_memory.indexer import Indexer
from agi_memory.store import FlatStore
from agi_memory.types import ChunkKind

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
    assert kinds[ChunkKind.PDF_PAGE] == 20  # 9 + 11 pages
    assert kinds[ChunkKind.IMAGE] == 26  # 3 PNGs in the repo + 23 pictures at the top level
    assert kinds[ChunkKind.CODE] > 100
    assert kinds[ChunkKind.MARKDOWN] >= 3


def test_syncing_again_does_no_work(indexed):
    store, indexer, first, _ = indexed
    count = store.count()
    second = indexer.sync("sample", SAMPLE)
    assert (second.added, second.updated) == (0, 0)
    assert second.unchanged == first.added
    assert store.count() == count
