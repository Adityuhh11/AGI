import pytest

from agi_memory.config import Settings
from agi_memory.embedding import FakeEmbedder
from agi_memory.indexer import Indexer
from agi_memory.search.searcher import Searcher
from agi_memory.store import FlatStore
from agi_memory.types import ChunkKind, Filters, SearchResult

DIM = 64


@pytest.fixture
def world(tmp_path):
    embedder = FakeEmbedder(DIM)
    store = FlatStore(tmp_path / "idx", model_id=embedder.model_id, dim=DIM)
    root = tmp_path / "src"
    (root / "Backend").mkdir(parents=True)
    (root / "docs").mkdir()
    (root / "Backend" / "jwt.js").write_text("function signRefreshToken() { refresh token jwt }")
    (root / "Backend" / "addcountries.js").write_text("async function addCountries() { seed }")
    (root / "docs" / "notes.md").write_text("# Notes\nthe quick brown fox jumps over the lazy dog")
    Indexer(store, embedder, Settings(index_dir=tmp_path / "idx")).sync("repo", root)
    yield Searcher(store, embedder), store
    store.close()


def test_a_semantic_style_query_finds_the_right_file(world):
    searcher, _ = world
    hits = searcher.search("how do I refresh the jwt token", k=3).hits
    assert hits[0].uri == "Backend/jwt.js"


def test_an_exact_identifier_is_found_through_the_keyword_side(world):
    searcher, _ = world
    top = searcher.search("addCountries", k=3).hits[0]
    assert top.uri == "Backend/addcountries.js"
    assert top.keyword_rank == 1


def test_typing_a_file_name_brings_that_file_to_the_top(world):
    searcher, _ = world
    assert searcher.search("notes.md", k=3).hits[0].uri == "docs/notes.md"
    assert searcher.search("JWT.JS", k=3).hits[0].uri == "Backend/jwt.js"


def test_hits_carry_the_documented_fields(world):
    searcher, _ = world
    hit = searcher.search("quick brown fox", k=1).hits[0]
    assert (hit.uri, hit.kind) == ("docs/notes.md", ChunkKind.MARKDOWN)
    assert hit.locator.startswith('{"lines":[')
    assert hit.score > 0
    assert hit.vector_score is not None
    assert hit.keyword_rank is not None
    assert "quick brown fox" in hit.snippet
    assert not hit.snippet.startswith("docs/notes.md")  # the path line is dropped
    assert len(hit.snippet) <= 240


def test_filters_reach_both_searches(world):
    searcher, _ = world
    hits = searcher.search("token", k=5, filters=Filters(uri_prefix="docs/")).hits
    assert hits
    assert all(h.uri.startswith("docs/") for h in hits)


def test_k_limits_the_results(world):
    searcher, _ = world
    assert len(searcher.search("token fox countries", k=2).hits) == 2


@pytest.mark.parametrize("query", ["", "   ", "the of and", "!!! ???", 'say "hi" * NEAR( x )'])
def test_blank_and_odd_queries_do_not_raise(world, query):
    searcher, _ = world
    assert isinstance(searcher.search(query).hits, list)


def test_a_five_thousand_character_query_works(world):
    searcher, _ = world
    assert isinstance(searcher.search("token " * 1000).hits, list)


def test_non_positive_k_returns_nothing(world):
    searcher, _ = world
    assert searcher.search("token", k=0) == SearchResult([], confident=False)


def test_results_are_deterministic(world):
    searcher, _ = world
    first = [h.id for h in searcher.search("token fox countries", k=5).hits]
    again = [h.id for h in searcher.search("token fox countries", k=5).hits]
    assert first == again


def test_a_chunk_deleted_between_ranking_and_lookup_is_skipped(world, monkeypatch):
    searcher, store = world
    real_get = store.get
    monkeypatch.setattr(
        store, "get", lambda ids: {k: v for k, v in real_get(ids).items() if k != ids[0]}
    )
    hits = searcher.search("refresh jwt token", k=3).hits
    assert len(hits) <= 2  # the top-ranked chunk vanished and is silently dropped


def test_a_close_semantic_match_is_confident_and_an_unrelated_query_is_not(world):
    _, store = world
    # The fake embedder's cosines are far below the real model's, so the cut-off is lowered.
    searcher = Searcher(store, FakeEmbedder(DIM), confident_cosine=0.4)
    close = searcher.search("quick brown fox", k=3)  # cosine about 0.51
    assert close.hits[0].vector_score and close.hits[0].vector_score >= 0.4
    assert close.confident
    unrelated = searcher.search("lasagna with extra mozzarella", k=3)  # cosines at most 0.29
    assert unrelated.hits
    assert not unrelated.confident


def test_a_file_name_is_confident_whatever_the_cosine(world):
    searcher, _ = world
    result = searcher.search("addcountries.js", k=3)
    assert result.hits[0].uri == "Backend/addcountries.js"
    assert result.confident


def test_an_empty_index_and_a_blank_query_are_not_confident(world):
    searcher, store = world
    assert searcher.search("   ").confident is False
    store.delete_source("repo")
    assert searcher.search("token") == SearchResult([], confident=False)
