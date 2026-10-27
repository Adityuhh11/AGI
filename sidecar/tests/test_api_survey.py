import threading

import pytest

from api_helpers import DIM, running_app
from omni_memory.embedding import FakeEmbedder


def test_a_survey_counts_what_indexing_would_read_by_kind(api, folder):
    (folder / "Backend" / "big.png").write_bytes(b"x" * 1000)
    body = api.post("/survey", json={"path": str(folder)}).json()
    assert body["routes"] == {
        "code": {"files": 2, "bytes": body["routes"]["code"]["bytes"]},
        "markdown": {"files": 1, "bytes": body["routes"]["markdown"]["bytes"]},
        "image": {"files": 1, "bytes": 1000},
    }
    assert body["denied"] == 1  # the .env file
    others = (body["ignored"], body["unsupported"], body["too_large"], body["unreadable"])
    assert others == (0, 0, 0, 0)


def test_a_survey_leaves_out_folders_like_indexing_does(api, folder):
    survey = api.post("/survey", json={"path": str(folder), "exclude": ["Backend"]}).json()
    assert list(survey["routes"]) == ["markdown"]
    assert survey["ignored"] == 1


def test_a_survey_does_not_read_the_files_it_counts(api, folder, monkeypatch):
    # Indexing reads each file with read_bytes; a survey only looks at the folder, so a file that
    # lives only in the cloud (OneDrive) is not fetched to be counted.
    def refuse(self):
        raise AssertionError("a survey must not read a file")

    monkeypatch.setattr("pathlib.Path.read_bytes", refuse)
    assert api.post("/survey", json={"path": str(folder)}).status_code == 200


@pytest.mark.parametrize("path", ["relative", "C:/not/here", "//nowhere.invalid/share"])
def test_a_survey_of_something_that_is_not_a_local_folder_is_a_400(api, path):
    assert api.post("/survey", json={"path": path}).status_code == 400


def test_a_survey_needs_the_token(api, folder):
    wrong = {"Authorization": "Bearer no"}
    assert api.post("/survey", json={"path": str(folder)}, headers=wrong).status_code == 401


def test_a_survey_answers_while_the_model_is_still_loading(tmp_path, folder):
    release = threading.Event()

    def slow_load():
        assert release.wait(10)
        return FakeEmbedder(DIM)

    with running_app(tmp_path / "idx", slow_load, wait=False) as client:
        assert client.post("/survey", json={"path": str(folder)}).status_code == 200
        release.set()
