import pytest
from fastapi.testclient import TestClient

from api_helpers import BASE, TOKEN

ROUTES = [
    ("get", "/health", None),
    ("post", "/search", {"query": "x"}),
    ("post", "/sources", {"path": "C:/x", "source_id": "s"}),
    ("get", "/jobs/job-1", None),
    ("delete", "/sources/s", None),
]
BAD_AUTH = [
    {},
    {"Authorization": "Bearer wrong"},
    {"Authorization": "Bearer"},
    {"Authorization": "Bearer "},
    {"Authorization": TOKEN},
    {"Authorization": f"Basic {TOKEN}"},
    {"Authorization": f"Bearer {TOKEN}x"},
    {"Authorization": f"Bearer {TOKEN[:-1]}"},
]


@pytest.fixture
def bare(api):
    return TestClient(api.app, base_url=BASE)


@pytest.mark.parametrize(("method", "path", "body"), ROUTES)
@pytest.mark.parametrize("headers", BAD_AUTH)
def test_every_route_rejects_a_bad_token(bare, method, path, body, headers):
    response = bare.request(method, path, json=body, headers=headers)
    assert response.status_code == 401
    assert TOKEN not in response.text


@pytest.mark.parametrize(("method", "path", "body"), ROUTES)
def test_authentication_beats_validation(bare, method, path, body):
    response = bare.request(
        method, path, json={"query": 5, "bogus": 1}, headers={"Authorization": "Bearer wrong"}
    )
    assert response.status_code == 401


@pytest.mark.parametrize(("method", "path", "body"), ROUTES)
def test_a_foreign_host_is_rejected_on_every_route_even_with_the_token(api, method, path, body):
    response = api.request(method, path, json=body, headers={"Host": "evil.example"})
    assert response.status_code == 400
