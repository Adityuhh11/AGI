import threading
from pathlib import Path

import pytest
from PIL import Image

from api_helpers import DIM, FailingEmbedder, GatedEmbedder, index, running_app, wait_ready
from helpers import wait_for
from omni_memory.embedding import FakeEmbedder


def test_adding_a_folder_indexes_it_and_reports_what_happened(api, folder):
    job = index(api, folder)
    assert (job["state"], job["error"], job["id"]) == ("done", None, "job-1")
    report = job["report"]
    assert (report["added"], report["denied"], report["errors"]) == (3, 1, [])
    assert job["files_done"] == job["files_total"] == 3
    assert api.get("/health").json()["embedded_items"] == 3


def test_syncing_the_same_folder_again_is_incremental(api, folder):
    index(api, folder)
    report = index(api, folder)["report"]
    assert (report["added"], report["updated"], report["unchanged"]) == (0, 0, 3)


def test_per_file_errors_are_listed_and_the_rest_is_indexed(api, folder):
    (folder / "broken.png").write_bytes(b"not an image")
    report = index(api, folder)["report"]
    assert [e["uri"] for e in report["errors"]] == ["broken.png"]
    assert report["added"] == 3


@pytest.mark.parametrize("path", ["relative/dir", "C:/definitely/not/here"])
def test_a_path_that_is_not_an_existing_absolute_folder_is_a_400(api, path):
    response = api.post("/sources", json={"path": path, "source_id": "repo"})
    assert response.status_code == 400


@pytest.mark.parametrize("path", ["//nowhere.invalid/share/x", "\\\\nowhere.invalid\\share\\x"])
def test_a_network_path_is_refused_without_touching_the_network(api, monkeypatch, path):
    def explode(self):
        raise AssertionError("is_dir() must not run on a network path")

    monkeypatch.setattr(Path, "is_dir", explode)
    response = api.post("/sources", json={"path": path, "source_id": "repo"})
    assert response.status_code == 400


def test_re_adding_a_source_from_another_folder_is_refused_and_keeps_the_index(
    api, folder, tmp_path
):
    index(api, folder)
    other = tmp_path / "other"
    other.mkdir()
    response = api.post("/sources", json={"path": str(other), "source_id": "repo"})
    assert response.status_code == 409
    assert "delete it first" in response.json()["detail"]["message"]
    assert api.get("/health").json()["embedded_items"] == 3
    assert api.delete("/sources/repo").json() == {"removed_chunks": 3}
    assert index(api, other)["state"] == "done"


def test_re_adding_a_source_with_a_differently_written_path_is_a_refresh(api, folder):
    index(api, folder)
    shouted = str(folder).upper().replace("\\", "/")
    response = api.post("/sources", json={"path": shouted, "source_id": "repo"})
    assert response.status_code == 202
    wait_for(lambda: api.get(f"/jobs/{response.json()['job_id']}").json()["state"] == "done")


def test_a_file_is_not_a_folder(api, folder):
    response = api.post("/sources", json={"path": str(folder / "notes.md"), "source_id": "repo"})
    assert response.status_code == 400


@pytest.mark.parametrize("source_id", ["", "../x", "a b", "-lead", ".hidden", "x" * 65, "a/b"])
def test_invalid_source_ids_are_rejected(api, folder, source_id):
    response = api.post("/sources", json={"path": str(folder), "source_id": source_id})
    assert response.status_code == 422


@pytest.mark.parametrize(
    "body",
    [
        {},
        {"path": "C:/x"},
        {"source_id": "r"},
        {"path": 5, "source_id": "r"},
        {"path": "C:/x", "source_id": "r", "extra": 1},
    ],
)
def test_malformed_bodies_are_rejected(api, body):
    assert api.post("/sources", json=body).status_code == 422


def test_a_non_json_body_is_rejected(api):
    response = api.post(
        "/sources", content=b"not json", headers={"Content-Type": "application/json"}
    )
    assert response.status_code == 422


def test_a_second_job_while_one_runs_is_refused_with_the_running_job_id(tmp_path, folder):
    gate = GatedEmbedder()
    with running_app(tmp_path / "idx", lambda: gate) as api:
        first = api.post("/sources", json={"path": str(folder), "source_id": "repo"})
        assert first.status_code == 202
        job_id = first.json()["job_id"]
        assert gate.entered.wait(5)
        busy = api.post("/sources", json={"path": str(folder), "source_id": "other"})
        assert busy.status_code == 409
        assert busy.json()["detail"]["job_id"] == job_id
        assert api.get(f"/jobs/{job_id}").json()["state"] == "running"
        gate.release.set()
        wait_for(lambda: api.get(f"/jobs/{job_id}").json()["state"] == "done")
        assert index(api, folder, "other")["state"] == "done"


def test_a_failing_job_reports_the_error_and_the_service_stays_up(tmp_path, folder):
    with running_app(tmp_path / "idx", FailingEmbedder) as api:
        job = index(api, folder)
        assert job["state"] == "failed"
        assert "gpu on fire" in job["error"]
        assert job["report"] is None
        assert api.get("/health").json()["ready"] is True


def test_unknown_jobs_are_a_404(api):
    assert api.get("/jobs/job-99").status_code == 404


def test_deleting_a_source_removes_its_chunks_and_is_idempotent(api, folder):
    index(api, folder)
    assert api.delete("/sources/repo").json() == {"removed_chunks": 3}
    assert api.get("/health").json()["embedded_items"] == 0
    assert api.delete("/sources/repo").json() == {"removed_chunks": 0}


def test_deleting_the_source_being_indexed_is_refused_but_others_are_fine(tmp_path, folder):
    gate = GatedEmbedder()
    with running_app(tmp_path / "idx", lambda: gate) as api:
        body = {"path": str(folder), "source_id": "repo"}
        job_id = api.post("/sources", json=body).json()["job_id"]
        assert gate.entered.wait(5)
        assert api.delete("/sources/repo").status_code == 409
        assert api.delete("/sources/other").json() == {"removed_chunks": 0}
        gate.release.set()
        wait_for(lambda: api.get(f"/jobs/{job_id}").json()["state"] == "done")


@pytest.mark.parametrize("source_id", ["-bad", "a b", "x" * 65])
def test_invalid_ids_in_the_delete_path_are_rejected(api, source_id):
    assert api.delete(f"/sources/{source_id}").status_code == 422


def test_everything_but_health_waits_for_the_model(tmp_path, folder):
    release = threading.Event()

    def slow_load():
        assert release.wait(10)
        return FakeEmbedder(DIM)

    body = {"path": str(folder), "source_id": "repo"}
    with running_app(tmp_path / "idx", slow_load, wait=False) as api:
        assert api.get("/health").json()["ready"] is False
        assert api.post("/sources", json=body).status_code == 503
        assert api.get("/jobs/job-1").status_code == 503
        assert api.delete("/sources/repo").status_code == 503
        release.set()
        wait_ready(api)
        assert index(api, folder)["state"] == "done"


def test_a_model_that_failed_to_load_keeps_answering_503_with_the_reason(tmp_path, folder):
    def broken():
        raise RuntimeError("no gpu driver")

    with running_app(tmp_path / "idx", broken, wait=False) as api:
        wait_for(lambda: api.get("/health").json()["error"])
        response = api.post("/sources", json={"path": str(folder), "source_id": "repo"})
        assert response.status_code == 503
        assert "no gpu driver" in response.json()["detail"]
        assert api.get("/jobs/job-1").status_code == 503


def test_secret_files_in_an_added_folder_are_counted_not_indexed(api, folder):
    job = index(api, folder)
    assert job["report"]["denied"] == 1
    assert job["report"]["added"] == 3  # the .env file is not among the indexed files


def test_listing_sources_shows_what_was_indexed(api, folder):
    assert api.get("/sources").json() == []
    index(api, folder)
    (source,) = api.get("/sources").json()
    assert (source["id"], source["root"]) == ("repo", str(folder))
    assert (source["files"], source["chunks"]) == (3, 3)
    assert source["last_scan_ns"] > 0
    api.delete("/sources/repo")
    assert api.get("/sources").json() == []


def test_listing_sources_works_while_the_model_is_still_loading(tmp_path):
    release = threading.Event()

    def slow_load():
        assert release.wait(10)
        return FakeEmbedder(DIM)

    with running_app(tmp_path / "idx", slow_load, wait=False) as client:
        assert client.get("/sources").status_code == 200  # reads the store only, not the model
        release.set()


def test_index_options_are_applied_and_a_bad_one_is_a_422(api, folder):
    Image.new("RGB", (32, 32), "blue").save(folder / "photo.png")
    body = {"path": str(folder), "source_id": "repo", "options": {"photos": "name"}}

    def sync(options):
        job_id = api.post("/sources", json={**body, "options": options}).json()["job_id"]
        return wait_for(
            lambda: (j := api.get(f"/jobs/{job_id}").json())["state"] != "running" and j
        )

    assert sync({"photos": "name"})["report"]["added"] == 4
    report = sync(
        {"photos": "content", "pdf_pages": "text", "videos": "content", "audio": "content"}
    )["report"]
    assert (report["updated"], report["unchanged"]) == (1, 3)  # only the picture changes
    for bad in [
        {"photos": "pixels"},
        {"pdf_pages": "some"},
        {"videos": "off"},
        {"audio": "on"},
        {"other": "x"},
    ]:
        assert api.post("/sources", json={**body, "options": bad}).status_code == 422, bad


def test_excluded_folders_are_left_out_and_a_bad_exclude_is_a_422(api, folder):
    (folder / "vendor").mkdir()
    (folder / "vendor" / "lib.txt").write_text("vendored text", encoding="utf-8")
    body = {"path": str(folder), "source_id": "repo", "exclude": ["vendor"]}
    job_id = api.post("/sources", json=body).json()["job_id"]
    wait_for(lambda: api.get(f"/jobs/{job_id}").json()["state"] != "running")
    assert api.get(f"/jobs/{job_id}").json()["report"]["added"] == 3  # the fixture's three files
    for bad in ["../outside", "/abs", "C:/abs", r"a\..\b"]:
        response = api.post("/sources", json={**body, "exclude": [bad]})
        assert response.status_code == 422, bad
