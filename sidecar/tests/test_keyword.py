import pytest

from helpers import make_chunk, unit_vectors
from omni_memory.store.keyword import build_match
from omni_memory.types import ChunkKind, Filters


@pytest.mark.parametrize(
    ("query", "expected"),
    [
        (
            "How do I refresh the access token?",
            '"refresh" OR "access" OR "token" OR "how do i refresh the access token"',
        ),
        ("addCountries", '"addcountries" OR "add" OR "countries"'),
        (
            "add_countries seed",
            '"add_countries" OR "seed" OR "add" OR "countries" OR "add_countries seed"',
        ),
        ("the of and", ""),
        ("", ""),
        ("   ", ""),
        ("!!! ??? ***", ""),
        ('say "hi" * NEAR( x )', '"say" OR "hi" OR "near" OR "say hi near x"'),
        ("token token TOKEN", '"token" OR "token token token"'),
        ("/refresh-token", '"refresh" OR "token" OR "refresh token"'),
        ("react-leaflet", '"react" OR "leaflet" OR "react leaflet"'),
        ("src/utils/jwt.js", '"src" OR "utils" OR "jwt" OR "js" OR "src utils jwt js"'),
        ("a.b.c", ""),
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


def test_a_punctuated_phrase_outranks_the_same_words_apart(store):
    together = make_chunk("t", "refresh-token handler")
    apart = make_chunk("p", "refresh the page then fetch a token")
    store.upsert([apart, together], unit_vectors(2))
    assert store.keyword_search("/refresh-token", 5)[0].id == together.id


def test_a_copied_sentence_matches_as_a_unit(store):
    exact = make_chunk("e", "throw new ApiError(400, Passwords do not match)")
    loose = make_chunk("l", "match the passwords; do not store them")
    store.upsert([loose, exact], unit_vectors(2))
    assert store.keyword_search("Passwords do not match", 5)[0].id == exact.id


def test_words_in_the_path_are_keyword_searchable_even_when_absent_from_the_text(store):
    named = make_chunk("locationService", "export const fetchAll = () => null")
    other = make_chunk("other", "export const fetchAll = () => null")
    store.upsert([other, named], unit_vectors(2))
    assert store.keyword_search("location service", 5)[0].id == named.id


def test_common_words_are_left_out_of_the_match():
    common = {"function", "const"}
    expected = '"addcountries" OR "add" OR "countries" OR "function addcountries const"'
    assert build_match("function addCountries const", common) == expected
    assert build_match("function const", common) == ""  # nothing selective left
    assert build_match("function") == '"function"'


def test_pasted_code_cannot_inflate_the_number_of_phrase_terms():
    query = " ".join(f"w{i}.x{i}" for i in range(300))
    match = build_match(query)
    phrases = [term for term in match.split(" OR ") if " " in term]
    assert len(phrases) == 8  # eight punctuated tokens; 600 words are too many for a whole phrase
    assert match.count(" OR ") < 32 + 8


def four_chunks(store):
    chunks = [make_chunk(n, f"{n}\nfunction body {n}") for n in ("alpha", "beta", "gamma", "delta")]
    store.upsert(chunks, unit_vectors(4))
    return chunks


def test_a_word_in_most_of_a_large_index_is_skipped_by_keyword_search(store, monkeypatch):
    monkeypatch.setattr("omni_memory.store.keyword._COMMON_MIN_DOCS", 0)
    monkeypatch.setattr("omni_memory.store.keyword._COMMON_FRACTION", 0.5)
    chunks = four_chunks(store)
    assert store.keyword_search("function", 10) == []  # in all four chunks: no signal
    assert store.keyword_search("function alpha", 10)[0].id == chunks[0].id


def test_small_indexes_never_skip_words(store):
    four_chunks(store)
    assert len(store.keyword_search("function", 10)) == 4
