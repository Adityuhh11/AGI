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


@pytest.fixture
def two_files(tmp_path):
    embedder = FakeEmbedder(DIM)
    store = FlatStore(tmp_path / "idx", model_id=embedder.model_id, dim=DIM)
    root = tmp_path / "src"
    root.mkdir()
    (root / "big.txt").write_text("\n".join(f"zebra line {i}" for i in range(130)))
    (root / "other.txt").write_text("zebra stands alone here")
    Indexer(store, embedder, Settings(index_dir=tmp_path / "idx")).sync("repo", root)
    yield Searcher(store, embedder), store
    store.close()


def test_hits_name_their_source(world):
    searcher, _ = world
    assert {hit.source_id for hit in searcher.search("token", k=5).hits} == {"repo"}


def test_a_file_with_many_matching_chunks_is_one_result_with_its_best_chunks(two_files):
    searcher, _ = two_files
    chunks = searcher.search("zebra", k=20).hits
    assert sum(hit.uri == "big.txt" for hit in chunks) >= 3  # the chunk list is mostly one file
    result = searcher.search_files("zebra", files=5, per_file=2)
    assert sorted(f.uri for f in result.files) == ["big.txt", "other.txt"]
    big = next(f for f in result.files if f.uri == "big.txt")
    assert len(big.chunks) == 2
    assert big.score == big.chunks[0].score >= big.chunks[1].score
    assert {f.source_id for f in result.files} == {"repo"}


def test_files_come_in_the_order_of_their_best_chunk(two_files):
    searcher, _ = two_files
    flat_order = list(dict.fromkeys(hit.uri for hit in searcher.search("zebra", k=60).hits))
    assert [f.uri for f in searcher.search_files("zebra").files] == flat_order


def test_the_number_of_files_and_chunks_per_file_is_capped(two_files):
    searcher, _ = two_files
    assert len(searcher.search_files("zebra", files=1).files) == 1
    assert all(len(f.chunks) == 1 for f in searcher.search_files("zebra", per_file=1).files)


def test_a_file_search_keeps_filters_confidence_and_blank_queries(two_files):
    searcher, _ = two_files
    only_other = searcher.search_files("zebra", filters=Filters(uri_prefix="other"))
    assert [f.uri for f in only_other.files] == ["other.txt"]
    assert searcher.search_files("other.txt").confident  # a file name is a deliberate match
    blank = searcher.search_files("   ")
    assert (blank.files, blank.confident) == ([], False)


def test_calibrating_makes_the_measured_noise_ceiling_the_threshold(world, monkeypatch):
    searcher, _ = world
    assert searcher.confident_cosine == 0.65
    monkeypatch.setattr("agi_memory.search.searcher.noise_ceiling", lambda store, embedder: 0.42)
    assert searcher.calibrate() == 0.42
    assert searcher.confident_cosine == 0.42


def test_an_index_too_small_to_measure_keeps_the_default_threshold(world, monkeypatch):
    searcher, _ = world
    monkeypatch.setattr("agi_memory.search.searcher.noise_ceiling", lambda store, embedder: None)
    assert searcher.calibrate() is None
    assert searcher.confident_cosine == 0.65
