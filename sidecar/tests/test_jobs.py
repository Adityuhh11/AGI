import pytest

from agi_memory.config import Settings
from agi_memory.embedding import FakeEmbedder
from agi_memory.indexer import Indexer
from agi_memory.jobs import JobBusyError, JobManager, JobState
from agi_memory.store import FlatStore
from api_helpers import FailingEmbedder, GatedEmbedder
from helpers import wait_for


@pytest.fixture
def make_manager(tmp_path):
    stores = []

    def make(embedder, keep=50):
        index = tmp_path / f"idx{len(stores)}"
        store = FlatStore(index, model_id=embedder.model_id, dim=embedder.dim)
        stores.append(store)
        settings = Settings(index_dir=index, model_id=embedder.model_id, dim=embedder.dim)
        return JobManager(Indexer(store, embedder, settings), keep=keep)

    yield make
    for store in stores:
        store.close()


@pytest.fixture
def folder(tmp_path):
    root = tmp_path / "src"
    root.mkdir()
    (root / "a.txt").write_text("alpha", encoding="utf-8")
    (root / "b.txt").write_text("beta", encoding="utf-8")
    return root


def finished(manager, job_id):
    return wait_for(lambda: (j := manager.get(job_id)) and j.state is not JobState.RUNNING and j)


def test_a_job_runs_to_completion_and_reports(make_manager, folder):
    manager = make_manager(FakeEmbedder(64))
    job = manager.submit("repo", folder)
    assert (job.id, job.source_id, job.root) == ("job-1", "repo", str(folder))
    done = finished(manager, job.id)
    assert done.state is JobState.DONE
    assert (done.files_done, done.files_total) == (2, 2)
    assert done.report is not None
    assert done.report.added == 2
    assert done.error is None


def test_a_second_job_while_one_runs_is_refused(make_manager, folder):
    gate = GatedEmbedder()
    manager = make_manager(gate)
    first = manager.submit("repo", folder)
    assert gate.entered.wait(5)
    assert manager.running_source() == "repo"
    with pytest.raises(JobBusyError) as busy:
        manager.submit("other", folder)
    assert busy.value.job_id == first.id
    assert manager.get(first.id).state is JobState.RUNNING
    gate.release.set()
    finished(manager, first.id)
    assert manager.running_source() is None
    assert manager.submit("other", folder).id == "job-2"
    finished(manager, "job-2")


def test_an_embedder_failure_fails_the_job_and_the_manager_survives(make_manager, folder):
    manager = make_manager(FailingEmbedder())
    job = finished(manager, manager.submit("repo", folder).id)
    assert job.state is JobState.FAILED
    assert job.error == "RuntimeError: gpu on fire"
    assert job.report is None
    assert finished(manager, manager.submit("repo", folder).id).state is JobState.FAILED


def test_a_missing_folder_fails_the_job_with_the_reason(make_manager, tmp_path):
    manager = make_manager(FakeEmbedder(64))
    job = finished(manager, manager.submit("repo", tmp_path / "gone").id)
    assert job.state is JobState.FAILED
    assert job.error is not None
    assert "FileNotFoundError" in job.error


def test_unknown_jobs_are_none(make_manager):
    assert make_manager(FakeEmbedder(64)).get("job-99") is None


def test_returned_jobs_are_snapshots(make_manager, folder):
    manager = make_manager(FakeEmbedder(64))
    job = manager.submit("repo", folder)
    finished(manager, job.id)
    job.state = JobState.FAILED  # a caller scribbling on its copy
    assert manager.get(job.id).state is JobState.DONE


def test_only_the_newest_jobs_are_remembered(make_manager, folder):
    manager = make_manager(FakeEmbedder(64), keep=2)
    for number in range(1, 5):
        finished(manager, manager.submit("repo", folder).id)
        assert manager.get(f"job-{number}") is not None
    assert manager.get("job-1") is None
    assert manager.get("job-2") is None
    assert manager.get("job-3") is not None
