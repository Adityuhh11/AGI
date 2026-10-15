import pytest

from agi_memory.types import Chunk, ChunkKind, Filters
from helpers import unit_vectors


def chunk(uri, locator="1", *, source_id="s", mtime_ns=1, kind=ChunkKind.TEXT):
    return Chunk(
        source_id=source_id,
        uri=uri,
        kind=kind,
        locator=f'{{"i":{locator}}}',
        content_hash=f"{uri}-{locator}",
        mtime_ns=mtime_ns,
        text="unrelated body",
    )


@pytest.fixture
def filled(store):
    chunks = [
        chunk("Backend/src/utils/jwt.js"),
        chunk("Backend/src/utils/emailService.js"),
        chunk("Backend/src/utils/emailService.js", "2"),
        chunk("docs/Screenshot 2026-07-29 233342.png", kind=ChunkKind.IMAGE),
        chunk("notes/readme.md", source_id="other"),
    ]
    store.upsert(chunks, unit_vectors(len(chunks)))
    return store, chunks


def uris(store, query, n=10, filters=None):
    found = {h.id for h in store.path_search(query, n, filters)}
    return sorted({c.uri for c in store.get(list(found)).values()})


def test_a_file_name_finds_every_chunk_of_that_file(filled):
    store, chunks = filled
    hits = store.path_search("emailService", 10)
    assert {h.id for h in hits} == {chunks[1].id, chunks[2].id}
    assert all(h.score > 0 for h in hits)


def test_the_match_is_a_case_insensitive_substring(filled):
    store, _ = filled
    assert uris(store, "EMAILSERV") == ["Backend/src/utils/emailService.js"]
    assert uris(store, "src/utils") == [
        "Backend/src/utils/emailService.js",
        "Backend/src/utils/jwt.js",
    ]
    assert uris(store, "Screenshot 2026-07-29 233342") == ["docs/Screenshot 2026-07-29 233342.png"]


@pytest.mark.parametrize(
    "query", ["", "  ", "jw", "ab", "no such file name", 'jwt"', 'say "hi" NEAR(']
)
def test_short_unmatched_and_odd_queries_return_a_list_not_an_error(filled, query):
    store, _ = filled
    assert isinstance(store.path_search(query, 10), list)
    if len(query.strip()) < 3 or query == "no such file name":
        assert store.path_search(query, 10) == []


def test_filters_and_the_limit_apply(filled):
    store, _ = filled
    assert uris(store, ".md") == ["notes/readme.md"]
    assert uris(store, ".md", filters=Filters(source_id="s")) == []
    assert uris(store, "src", filters=Filters(uri_prefix="Backend/src/utils/j")) == [
        "Backend/src/utils/jwt.js"
    ]
    assert uris(store, "docs", filters=Filters(kinds=frozenset({ChunkKind.IMAGE}))) == [
        "docs/Screenshot 2026-07-29 233342.png"
    ]
    assert uris(store, "docs", filters=Filters(kinds=frozenset())) == []
    assert len(store.path_search("src", 2)) == 2
    assert store.path_search("src", 0) == []


def test_deleted_and_replaced_files_leave_the_path_index(filled):
    store, chunks = filled
    store.delete([chunks[0].id])
    assert uris(store, "jwt") == []
    renamed = chunk("Backend/src/utils/tokens.js", "9")
    store.upsert([renamed], unit_vectors(1, seed=5))
    assert uris(store, "tokens") == ["Backend/src/utils/tokens.js"]


def test_results_are_ordered_deterministically(filled):
    store, _ = filled
    first = [h.id for h in store.path_search("src", 10)]
    assert first == [h.id for h in store.path_search("src", 10)]
