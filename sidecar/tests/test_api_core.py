import threading

import pytest
from fastapi.testclient import TestClient

from api_helpers import BASE, DIM, TOKEN, running_app, wait_ready
from helpers import wait_for
from omni_memory.api.app import create_app
from omni_memory.config import Settings
from omni_memory.embedding import FakeEmbedder


def test_health_describes_a_ready_engine(api):
    body = api.get("/health").json()
    assert body["ready"] is True
    assert body["model"] == FakeEmbedder(DIM).model_id
    assert (body["device"], body["embedded_items"], body["error"]) == ("cpu", 0, None)
    assert body["version"]


def test_health_answers_while_the_model_is_still_loading(tmp_path):
    release = threading.Event()

    def slow_load():
        assert release.wait(10)
        return FakeEmbedder(DIM)

    with running_app(tmp_path / "idx", slow_load, wait=False) as client:
        body = client.get("/health").json()
        assert (body["ready"], body["device"], body["error"]) == (False, None, None)
        release.set()
        wait_ready(client)
        assert client.get("/health").json()["device"] == "cpu"


def test_a_model_that_fails_to_load_is_reported_not_fatal(tmp_path):
    def broken():
        raise RuntimeError("no gpu driver")

    with running_app(tmp_path / "idx", broken, wait=False) as client:
        body = wait_for(lambda: (b := client.get("/health").json())["error"] and b)
        assert body["ready"] is False
        assert "no gpu driver" in body["error"]


def test_an_embedder_that_does_not_match_the_settings_is_reported(tmp_path):
    with running_app(tmp_path / "idx", lambda: FakeEmbedder(32), wait=False) as client:
        body = wait_for(lambda: (b := client.get("/health").json())["error"] and b)
        assert body["ready"] is False
        assert "dim" in body["error"]


def test_a_foreign_host_header_is_rejected_even_with_the_right_token(api):
    assert api.get("/health", headers={"Host": "evil.example"}).status_code == 400
    assert api.get("/health", headers={"Host": "evil.example:7861"}).status_code == 400


@pytest.mark.parametrize("host", ["127.0.0.1:7861", "localhost:7861", "localhost", "127.0.0.1"])
def test_local_host_headers_are_accepted(api, host):
    assert api.get("/health", headers={"Host": host}).status_code == 200


def test_health_needs_the_token(api):
    bare = TestClient(api.app, base_url=BASE)
    assert bare.get("/health").status_code == 401
    assert bare.get("/health", headers={"Authorization": "Bearer wrong"}).status_code == 401
    assert TOKEN not in bare.get("/health").text


def test_documentation_pages_are_not_served(api):
    for path in ("/docs", "/redoc", "/openapi.json"):
        assert api.get(path).status_code == 404


@pytest.mark.parametrize("token", ["", "short", "x" * 15])
def test_a_missing_or_short_token_refuses_to_build_the_app(tmp_path, token):
    settings = Settings(index_dir=tmp_path / "idx", dim=DIM)
    with pytest.raises(ValueError, match="token"):
        create_app(settings, token, lambda: FakeEmbedder(DIM))


def test_the_index_survives_a_restart(tmp_path):
    with running_app(tmp_path / "idx") as first:
        assert first.get("/health").json()["embedded_items"] == 0
    with running_app(tmp_path / "idx") as second:
        assert second.get("/health").json()["ready"] is True
