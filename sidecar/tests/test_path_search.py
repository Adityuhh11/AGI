import pytest

from helpers import unit_vectors
from omni_memory.types import Chunk, ChunkKind, Filters


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


def uris(store, query, filters=None):
    found = {h.id for h in store.path_search(query, filters)}
    return sorted({c.uri for c in store.get(list(found)).values()})


def test_a_file_name_finds_every_chunk_of_that_file_with_the_same_score(filled):
    store, chunks = filled
    hits = store.path_search("emailService")
    assert {h.id for h in hits} == {chunks[1].id, chunks[2].id}
    assert {h.score for h in hits} == {1.0}  # the embedding orders them, not the path length


def test_the_match_is_a_case_insensitive_substring(filled):
    store, _ = filled
    assert uris(store, "EMAILSERV") == ["Backend/src/utils/emailService.js"]
    assert uris(store, "src/utils") == [
        "Backend/src/utils/emailService.js",
        "Backend/src/utils/jwt.js",
    ]
    assert uris(store, "Screenshot 2026-07-29 233342") == ["docs/Screenshot 2026-07-29 233342.png"]


@pytest.mark.parametrize(
    "query",
    ["", "  ", "jw", "ab", "no such file name", 'jwt"', 'say "hi" NEAR(', "x" * 5000],
)
def test_short_unmatched_and_odd_queries_return_a_list_not_an_error(filled, query):
    store, _ = filled
    assert store.path_search(query) == []


def test_filters_apply(filled):
    store, _ = filled
    assert uris(store, ".md") == ["notes/readme.md"]
    assert uris(store, ".md", Filters(source_id="s")) == []
    prefix = Filters(uri_prefix="Backend/src/utils/j")
    assert uris(store, "src", prefix) == ["Backend/src/utils/jwt.js"]
    assert uris(store, "docs", Filters(kinds=frozenset({ChunkKind.IMAGE}))) == [
        "docs/Screenshot 2026-07-29 233342.png"
    ]
    assert uris(store, "docs", Filters(kinds=frozenset())) == []


def test_a_fragment_found_in_too_many_files_is_ignored(filled, monkeypatch):
    store, _ = filled
    assert len(uris(store, "src")) == 2
    monkeypatch.setattr("omni_memory.store.keyword._MAX_PATH_FILES", 1)
    assert store.path_search("src") == []  # two files match: too common to mean a name
    assert len(store.path_search("jwt.js")) == 1  # a specific name still works
    # the limit is counted inside the searched scope, so a filter can make a fragment specific
    assert len(store.path_search("src", Filters(uri_prefix="Backend/src/utils/j"))) == 1


def test_a_fragment_found_in_too_many_chunks_is_ignored(filled, monkeypatch):
    store, _ = filled
    monkeypatch.setattr("omni_memory.store.keyword._MAX_PATH_CHUNKS", 1)
    assert store.path_search("emailService") == []  # one file, but two chunks
    assert len(store.path_search("jwt.js")) == 1


def test_deleted_and_replaced_files_leave_the_path_index(filled):
    store, chunks = filled
    store.delete([chunks[0].id])
    assert uris(store, "jwt") == []
    renamed = chunk("Backend/src/utils/tokens.js", "9")
    store.upsert([renamed], unit_vectors(1, seed=5))
    assert uris(store, "tokens") == ["Backend/src/utils/tokens.js"]


def test_results_are_ordered_by_id(filled):
    store, _ = filled
    ids = [h.id for h in store.path_search("src")]
    assert ids == sorted(ids)


def test_a_nul_character_in_a_query_is_ignored_and_does_not_fail(filled):
    store, _ = filled
    assert store.path_search("emailService\x00") == store.path_search("emailService") != []
    assert store.path_search("jw\x00t.js") == store.path_search("jwt.js") != []
    assert store.path_search("\x00\x00\x00") == []
