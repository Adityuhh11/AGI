import pytest

from agi_memory.store.keyword import build_match
from agi_memory.types import ChunkKind, Filters
from helpers import make_chunk, unit_vectors


@pytest.mark.parametrize(
    ("query", "expected"),
    [
        ("How do I refresh the access token?", '"refresh" OR "access" OR "token"'),
        ("addCountries", '"addcountries" OR "add" OR "countries"'),
        ("add_countries seed", '"add_countries" OR "seed" OR "add" OR "countries"'),
        ("the of and", ""),
        ("", ""),
        ("   ", ""),
        ("!!! ??? ***", ""),
        ('say "hi" * NEAR( x )', '"say" OR "hi" OR "near"'),
        ("token token TOKEN", '"token"'),
        ("a b c", ""),
    ],
)
def test_build_match(query, expected):
    assert build_match(query) == expected


def test_build_match_caps_the_number_of_words():
    query = " ".join(f"word{i}" for i in range(100))
    assert build_match(query).count(" OR ") == 31


def fill(store):
    chunks = [
        make_chunk("a", "Backend/addcountries.js\nconst addCountries = seed countries csv"),
        make_chunk("b", "Backend/jwt.js\nrefresh token generate access token", kind=ChunkKind.CODE),
        make_chunk("c", "notes.md\nthe quick brown fox refresh the token", source_id="docs"),
    ]
    store.upsert(chunks, unit_vectors(3))
    return {c.uri: c.id for c in chunks}


def test_ranking_prefers_more_relevant_chunks(store):
    ids = fill(store)
    hits = store.keyword_search("refresh access token", 5)
    assert hits[0].id == ids["b.txt"]
    assert {h.id for h in hits} == {ids["b.txt"], ids["c.txt"]}
    assert hits[0].score > hits[1].score > 0


def test_identifiers_are_findable_by_their_words_and_whole(store):
    ids = fill(store)
    assert store.keyword_search("countries", 5)[0].id == ids["a.txt"]
    assert store.keyword_search("addcountries", 5)[0].id == ids["a.txt"]


def test_file_names_in_the_text_are_searchable(store):
    ids = fill(store)
    assert store.keyword_search("jwt.js", 5)[0].id == ids["b.txt"]


def test_filters_apply_to_keyword_search(store):
    ids = fill(store)
    only_docs = store.keyword_search("token", 5, Filters(source_id="docs"))
    assert [h.id for h in only_docs] == [ids["c.txt"]]
    only_code = store.keyword_search("token", 5, Filters(kinds=frozenset({ChunkKind.CODE})))
    assert [h.id for h in only_code] == [ids["b.txt"]]
    assert store.keyword_search("token", 5, Filters(kinds=frozenset())) == []


def test_deleted_and_replaced_chunks_leave_the_keyword_index(store):
    ids = fill(store)
    store.delete([ids["b.txt"]])
    assert [h.id for h in store.keyword_search("token", 5)] == [ids["c.txt"]]


@pytest.mark.parametrize(
    "query", ["", "   ", "the of and", "!!!", 'say "hi" * NEAR( x ) OR AND NOT', "x" * 5000]
)
def test_odd_queries_never_raise(store, query):
    fill(store)
    assert isinstance(store.keyword_search(query, 5), list)


def test_empty_results_and_limits(store):
    fill(store)
    assert store.keyword_search("nonexistentword", 5) == []
    assert store.keyword_search("token", 0) == []
    assert len(store.keyword_search("token", 1)) == 1


def test_ties_are_broken_by_id(store):
    chunks = [make_chunk(name, "identical words here") for name in ("zulu", "alpha", "mike")]
    store.upsert(chunks, unit_vectors(3))
    hits = store.keyword_search("identical words", 5)
    assert [h.id for h in hits] == sorted(c.id for c in chunks)
