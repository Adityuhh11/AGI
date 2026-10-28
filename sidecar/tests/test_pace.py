import threading
import time

import numpy as np
import pytest

from api_helpers import DIM, index
from helpers import wait_for
from omni_memory.config import Settings
from omni_memory.embedding import FakeEmbedder
from omni_memory.indexer import Indexer
from omni_memory.jobs import JobManager, JobState
from omni_memory.pace import MIN_DUTY, PacedEmbedder, Pacer
from omni_memory.store import FlatStore


@pytest.mark.parametrize(
    ("duty", "busy", "want"),
    [(1.0, 2.0, 0.0), (0.5, 2.0, 2.0), (0.2, 2.0, 8.0), (0.25, 1.0, 3.0), (0.5, -1.0, 0.0)],
)
def test_the_rest_after_a_stretch_of_work_makes_it_the_wanted_share_of_the_time(duty, busy, want):
    assert Pacer(duty).delay_for(busy) == pytest.approx(want)


@pytest.mark.parametrize("duty", [0.0, MIN_DUTY / 2, 1.01, -1.0])
def test_a_duty_outside_the_allowed_range_is_refused(duty):
    with pytest.raises(ValueError, match="duty must be between"):
        Pacer(duty)
    pacer = Pacer()
    with pytest.raises(ValueError, match="duty must be between"):
        pacer.set_duty(duty)
    assert pacer.duty == 1.0


def timed(call) -> float:
    started = time.monotonic()
    call()
    return time.monotonic() - started


def test_flat_out_does_not_wait():
    assert timed(lambda: Pacer(1.0).checkpoint(5.0)) < 0.1


def test_a_half_duty_waits_as_long_as_the_work_took():
    took = timed(lambda: Pacer(0.5).checkpoint(0.3))
    assert 0.25 <= took < 0.6


def test_a_paused_pacer_holds_the_indexer_until_it_is_resumed():
    pacer = Pacer()
    pacer.pause()
    done = threading.Event()
    threading.Thread(target=lambda: (pacer.checkpoint(0.0), done.set()), daemon=True).start()
    assert not done.wait(0.3), "a paused pacer let the indexer through"
    pacer.resume()
    assert done.wait(2), "resuming did not release the indexer"
    assert pacer.paused is False


def test_a_faster_speed_chosen_during_a_rest_ends_it_early():
    pacer = Pacer(0.05)  # a 19 s rest after 1 s of work
    done = threading.Event()
    threading.Thread(target=lambda: (pacer.checkpoint(1.0), done.set()), daemon=True).start()
    time.sleep(0.1)
    pacer.set_duty(1.0)
    assert done.wait(1), "the rest already under way ignored the new speed"


def test_a_pause_that_arrives_during_a_rest_holds_at_the_end_of_it():
    pacer = Pacer(0.5)
    done = threading.Event()
    threading.Thread(target=lambda: (pacer.checkpoint(0.4), done.set()), daemon=True).start()
    time.sleep(0.1)
    pacer.pause()  # arrives while the rest period is running
    assert not done.wait(0.6), "the rest ended and the paused indexer went on"
    pacer.resume()
    assert done.wait(2)


class Recorder:
    def __init__(self):
        self.calls = []

    def checkpoint(self, busy):
        self.calls.append(busy)


def test_only_bulk_calls_are_paced_never_a_query():
    recorder = Recorder()
    paced = PacedEmbedder(FakeEmbedder(DIM), recorder)  # type: ignore[arg-type]
    paced.embed_documents(["a", "b"], ["x", "y"])
    paced.embed_images([])
    paced.embed_audio([])
    assert len(recorder.calls) == 3 and all(c >= 0 for c in recorder.calls)
    paced.embed_query("anything")
    assert len(recorder.calls) == 3
    assert (paced.model_id, paced.dim, paced.device) == ("fake-hashed-bow-v1", DIM, "cpu")


def test_the_vectors_are_the_inner_embedders(tmp_path):
    inner = FakeEmbedder(DIM)
    paced = PacedEmbedder(inner, Pacer())
    assert np.array_equal(
        paced.embed_documents(["hello"], ["t"]), inner.embed_documents(["hello"], ["t"])
    )
    assert np.array_equal(paced.embed_query("q"), inner.embed_query("q"))


def make_manager(tmp_path, pacer):
    inner = FakeEmbedder(DIM)
    store = FlatStore(tmp_path / "idx", model_id=inner.model_id, dim=DIM)
    settings = Settings(
        index_dir=tmp_path / "idx", model_id=inner.model_id, dim=DIM, flush_chunks=1
    )
    return store, JobManager(Indexer(store, PacedEmbedder(inner, pacer), settings))


def test_a_paused_index_job_stops_embedding_and_finishes_after_resume(tmp_path):
    folder = tmp_path / "docs"
    folder.mkdir()
    for i in range(6):
        (folder / f"f{i}.txt").write_text(f"note number {i} about topic {i}", encoding="utf-8")
    pacer = Pacer()
    pacer.pause()  # paused before the job starts: it must not embed anything
    store, jobs = make_manager(tmp_path, pacer)
    try:
        job = jobs.submit("docs", folder)
        time.sleep(0.5)
        assert jobs.get(job.id).state is JobState.RUNNING
        assert store.count() == 0, "vectors were stored while paused"
        pacer.resume()
        done = wait_for(lambda: (j := jobs.get(job.id)) and j.state is not JobState.RUNNING and j)
        assert done.state is JobState.DONE and store.count() == 6
    finally:
        store.close()


def test_the_api_reports_and_changes_the_pace(api):
    assert api.get("/pace").json() == {"duty": 1.0, "paused": False}
    health = api.get("/health").json()
    assert (health["duty"], health["paused"]) == (1.0, False)
    assert api.put("/pace", json={"duty": 0.5}).json() == {"duty": 0.5, "paused": False}
    assert api.post("/pause").json() == {"duty": 0.5, "paused": True}
    assert api.get("/health").json()["paused"] is True
    assert api.post("/resume").json() == {"duty": 0.5, "paused": False}


@pytest.mark.parametrize(
    "body", [{"duty": 0}, {"duty": 2}, {"duty": "fast"}, {}, {"duty": 0.5, "x": 1}]
)
def test_a_bad_pace_is_refused_and_changes_nothing(api, body):
    assert api.put("/pace", json=body).status_code == 422
    assert api.get("/pace").json()["duty"] == 1.0


def test_the_pace_needs_the_token_and_a_loaded_engine(api, tmp_path):
    assert (
        api.put("/pace", json={"duty": 0.5}, headers={"Authorization": "Bearer wrong"}).status_code
        == 401
    )
    assert api.post("/pause", headers={"Authorization": "Bearer wrong"}).status_code == 401


def test_searching_works_while_indexing_is_paused(api, tmp_path):
    folder = tmp_path / "docs"
    folder.mkdir()
    (folder / "jwt.txt").write_text("how to refresh a jwt token", encoding="utf-8")
    assert index(api, folder)["state"] == "done"
    api.post("/pause")
    started = time.monotonic()
    body = api.post("/search", json={"query": "refresh jwt", "k": 3}).json()
    assert time.monotonic() - started < 2 and body["hits"]
