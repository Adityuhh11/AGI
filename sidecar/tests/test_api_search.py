import pytest

from api_helpers import SlowEmbedder, index, running_app
from helpers import wait_for


def search(client, query, **extra):
    return client.post("/search", json={"query": query, **extra})


@pytest.fixture
def indexed(api, folder):
    index(api, folder)
    return api


def test_a_search_finds_the_right_file_and_returns_the_documented_fields(indexed):
    hits = search(indexed, "how do I refresh the jwt token", k=3).json()["hits"]
    assert hits[0]["uri"] == "Backend/jwt.js"
    assert set(hits[0]) == {
        "id", "uri", "kind", "locator", "score", "vector_score", "keyword_rank", "snippet",
    }  # fmt: skip
    assert hits[0]["kind"] == "code"
    assert hits[0]["locator"].startswith('{"lines":[')
    assert "refresh" in hits[0]["snippet"]


def test_the_response_says_whether_the_best_hit_looks_like_an_answer(indexed):
    good = search(indexed, "jwt.js", k=3).json()  # a file name is a deliberate match
    assert good["confident"] is True
    unrelated = search(indexed, "lasagna with extra mozzarella", k=3).json()
    assert unrelated["hits"]
    assert unrelated["confident"] is False


def test_an_exact_identifier_is_found_by_its_name(indexed):
    assert (
        search(indexed, "addCountries", k=1).json()["hits"][0]["uri"] == "Backend/addcountries.js"
    )


def test_results_are_deterministic_and_k_limits_them(indexed):
    first = search(indexed, "token fox countries", k=2).json()
    assert search(indexed, "token fox countries", k=2).json() == first
    assert len(first["hits"]) == 2


def test_filters_narrow_the_results(indexed):
    kinds = search(indexed, "token", filters={"kinds": ["markdown"]}).json()["hits"]
    assert kinds
    assert {h["kind"] for h in kinds} == {"markdown"}
    prefixed = search(indexed, "token", filters={"uri_prefix": "Backend/"}).json()["hits"]
    assert prefixed
    assert all(h["uri"].startswith("Backend/") for h in prefixed)
    assert search(indexed, "token", filters={"source_id": "nope"}).json() == {
        "hits": [],
        "confident": False,
    }
    assert search(indexed, "token", filters={"modified_after_ns": 2**62}).json()["hits"] == []
    assert search(indexed, "token", filters={"source_id": "repo"}).json()["hits"]


def test_secret_files_are_not_searchable_through_the_api(indexed):
    for query in ("hunter2", "JWT_SECRET"):
        uris = [h["uri"] for h in search(indexed, query).json()["hits"]]
        assert not [u for u in uris if ".env" in u]


@pytest.mark.parametrize(
    "query",
    [
        "",
        "   ",
        "!!! ???",
        "the of and",
        'say "hi" * NEAR( x ) OR AND NOT',
        "x" * 4000,
        "token " * 600,
    ],
)
def test_odd_queries_return_a_list_not_an_error(indexed, query):
    response = search(indexed, query)
    assert response.status_code == 200
    assert isinstance(response.json()["hits"], list)


@pytest.mark.parametrize("query", ["", "   "])
def test_blank_queries_have_no_hits(indexed, query):
    assert search(indexed, query).json() == {"hits": [], "confident": False}


@pytest.mark.parametrize(
    "body",
    [
        {"query": "x" * 4001},
        {"query": "x", "k": 0},
        {"query": "x", "k": 101},
        {"query": "x", "k": "ten"},
        {"query": 5},
        {},
        {"query": "x", "bogus": 1},
        {"query": "x", "filters": {"bogus": 1}},
        {"query": "x", "filters": {"kinds": ["bogus"]}},
        {"query": "x", "filters": {"modified_after_ns": -1}},
        {"query": "x", "filters": {"source_id": "../x"}},
    ],
)
def test_invalid_search_bodies_are_rejected(indexed, body):
    assert indexed.post("/search", json=body).status_code == 422


def test_searching_an_empty_index_returns_nothing(api):
    assert search(api, "anything").json() == {"hits": [], "confident": False}


def test_searching_works_while_a_job_is_indexing(tmp_path):
    big = tmp_path / "big"
    big.mkdir()
    for number in range(30):
        (big / f"f{number}.txt").write_text(f"alpha beta gamma file{number}")
    with running_app(tmp_path / "idx", SlowEmbedder, flush_chunks=1) as api:
        job_id = api.post("/sources", json={"path": str(big), "source_id": "big"}).json()["job_id"]
        during = 0
        while api.get(f"/jobs/{job_id}").json()["state"] == "running":
            assert search(api, "alpha").status_code == 200
            during += 1
        assert during >= 3
        wait_for(lambda: api.get(f"/jobs/{job_id}").json()["state"] == "done")
        assert len(search(api, "alpha", k=30).json()["hits"]) == 30
