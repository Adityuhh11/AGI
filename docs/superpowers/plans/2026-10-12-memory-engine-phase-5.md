# Memory Engine, Phase 5 (HTTP API) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Put the memory engine behind an authenticated localhost HTTP API and a process entry point, so the Go launcher can start it, wait for it, index folders, search, and stop it.

**Architecture:** `api/` is a thin FastAPI layer over code that already exists. `create_app` opens the store immediately and loads the embedding model in a background thread, so the port answers `/health` within a second while the model takes ~10 s. A new `JobManager` runs `Indexer.sync` in one background thread at a time and exposes progress. A new `SerializedEmbedder` lets the job thread and request threads share one model safely. `python -m omni_memory` reads the token and port from the environment and runs uvicorn on `127.0.0.1`.

**Tech Stack:** Python >= 3.12, FastAPI, uvicorn, pydantic (comes with FastAPI), `httpx2` (dev, for the test client), plus everything from phases 0-4.

**Spec:** `docs/superpowers/specs/2026-10-12-memory-engine-design.md` (section 8 and phase 5 of section 13; sections 3, 4, 9, 11). Previous plans: `2026-10-12-memory-engine-phases-0-2.md`, `2026-10-12-memory-engine-phases-3-4.md` (both merged).

## Global Constraints

- Python `>=3.12`, `uv`; run commands from `C:\dev\VS_CODE\Omni\sidecar` unless a step says otherwise. Run Bash tool calls one at a time (parallel calls interleave the working directory). In Python snippets written through a shell heredoc, avoid backslash escapes (use the Edit tool for code containing `\n`).
- The API binds `127.0.0.1` only (default port 7861). **Every** route, including `/health`, needs `Authorization: Bearer <token>`; the token is read from env `OMNI_SIDECAR_TOKEN` and must be at least 16 characters; the server refuses to start without one. Requests whose `Host` header is not `127.0.0.1` or `localhost` (any port) are rejected with 400 before authentication is looked at. The token is compared in constant time and never logged or echoed.
- Auto-generated docs (`/docs`, `/redoc`, `/openapi.json`) are disabled: they would be unauthenticated pages.
- GPU = models only; index storage and search stay on the CPU. Embedding model `google/embeddinggemma-2`, 256-d float32 unit vectors.
- One index job at a time. Everything except `/health` answers 503 until the model is loaded.
- Nothing leaves the machine; no telemetry. Never read, index, log, print or commit `.env*` files or anything under `Sample Data/`.
- Determinism: job ids are sequential (`job-1`, `job-2`, ...), search results keep the phase 4 tie-breaking, tests use `FakeEmbedder` (no GPU, no real model); real-model tests carry `@pytest.mark.model`.
- Quality gate for every task: `uv run ruff check .`, `uv run ruff format --check .`, `uv run pyright`, `uv run pytest` all clean before commit (check real exit status; do not hide it behind `| tail`).
- Conventional commits, one logical change each, tests with the code they cover, docs in the same commit as the behaviour. **Do not add any `Co-Authored-By` trailer or other AI attribution to commit messages** (owner's instruction; it overrides any harness default).

## Review Focus

Inputs and conditions the spec implies but its listed tests do not exercise; each has a test in the owning task.

1. Authentication: a missing, wrong, empty, scheme-less or `Basic` token gets 401 on **every** route including `/health`, also when the body is invalid (401 wins over 422); the token never appears in a response body; the server will not start with a missing or short token (Tasks 3, 5, 6).
2. DNS rebinding: a request with `Host: evil.com` is rejected even with the correct token, on every route; `localhost:7861` and `127.0.0.1:7861` work (Tasks 3, 5).
3. Concurrency: searching works while an index job is running (no deadlock, no torn results); a second `POST /sources` during a job gets 409 with the running job's id; deleting the source being indexed gets 409; the model-load window answers `/health` with `ready: false` and everything else with 503 (Tasks 1, 2, 4, 5).
4. Hostile input: relative path, missing folder, a file instead of a folder, path/source ids with `..`, spaces or 65 characters, `k` of 0 / 101 / a string, unknown JSON fields, a 4001-character query, empty / punctuation-only / FTS-operator queries, non-JSON bodies, invalid chunk kinds in filters (Tasks 4, 5).
5. Failure containment: an embedder that raises, a folder deleted before the job starts, and a corrupt file each end as a `failed` job or a per-file error while the service keeps answering; a model that fails to load shows up as `error` in `/health` instead of killing the process; the job table stays bounded (Tasks 2, 3, 4).

## File Structure

```
sidecar/
  pyproject.toml                          (modify: fastapi, uvicorn, httpx2)
  src/omni_memory/
    embedding/base.py                     (modify: `device` on the Embedder protocol)
    embedding/fake.py                     (modify: `device`)
    embedding/serialized.py               (create: SerializedEmbedder)
    eval/runner.py                        (modify: use embedder.device)
    jobs.py                               (create: Job, JobState, JobBusyError, JobManager)
    api/__init__.py  deps.py  schemas.py  routes.py  app.py   (create)
    __main__.py                           (create: process entry point)
  tests/
    helpers.py                            (modify: wait_for)
    api_helpers.py                        (create: embedders that misbehave, running_app)
    conftest.py                           (modify: api, folder fixtures)
    test_serialized_embedder.py  test_jobs.py  test_api_core.py  test_api_sources.py
    test_api_search.py  test_api_auth.py  test_main.py  test_api_process.py
AGENTS.md, docs/{api.md, ARCHITECTURE.md, STATUS.md, README.md}
```

Deviations from the spec made while planning (listed again in `docs/api.md`): `/health` also returns `error` (why the model failed to load); `DELETE /sources/{id}` returns `{removed_chunks}` and is idempotent; `/docs` is disabled; IPv6 hosts (`[::1]`) are rejected, clients must use `127.0.0.1`; `IndexJob` in the spec's module table is `JobManager`/`Job`; the `Embedder` protocol gains `device`; `modified_after` is `modified_after_ns` (integer nanoseconds, like the store).

---

### Task 1: Dependencies, `Embedder.device`, `SerializedEmbedder`

**Files:**
- Modify: `sidecar/pyproject.toml`, `sidecar/src/omni_memory/embedding/base.py`, `sidecar/src/omni_memory/embedding/fake.py`, `sidecar/src/omni_memory/eval/runner.py`, `sidecar/tests/test_eval.py`, `sidecar/tests/helpers.py`
- Create: `sidecar/src/omni_memory/embedding/serialized.py`
- Test: `sidecar/tests/test_serialized_embedder.py`, `sidecar/tests/test_fake_embedder.py` (append)

**Interfaces:**
- Produces `Embedder.device` (read-only `str` property; `GemmaEmbedder` already has it, `FakeEmbedder.device == "cpu"`).
- Produces `omni_memory.embedding.serialized.SerializedEmbedder(inner: Embedder)`: implements `Embedder`; `embed_documents`, `embed_images` and `embed_query` run one at a time behind a lock; `model_id`, `dim`, `device` delegate.
- Produces `tests/helpers.wait_for(predicate: Callable[[], T], timeout: float = 10.0, interval: float = 0.01) -> T`: polls until `predicate()` is truthy and returns that value; raises `AssertionError` on timeout.

- [ ] **Step 1: Write the failing tests**

`sidecar/tests/test_serialized_embedder.py`:

```python
import threading
import time

import numpy as np

from omni_memory.embedding import FakeEmbedder
from omni_memory.embedding.serialized import SerializedEmbedder


class Probe(FakeEmbedder):
    """Records how many calls are inside the embedder at the same moment."""

    def __init__(self) -> None:
        super().__init__(16)
        self.active = 0
        self.peak = 0
        self._guard = threading.Lock()

    def _inside(self) -> None:
        with self._guard:
            self.active += 1
            self.peak = max(self.peak, self.active)
        time.sleep(0.005)
        with self._guard:
            self.active -= 1

    def embed_documents(self, texts):
        self._inside()
        return super().embed_documents(texts)

    def embed_query(self, text):
        self._inside()
        return super().embed_query(text)


def hammer(embedder) -> None:
    def work():
        embedder.embed_documents(["a"])
        embedder.embed_query("b")

    threads = [threading.Thread(target=work) for _ in range(8)]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join()


def test_without_the_wrapper_calls_do_overlap():
    probe = Probe()
    hammer(probe)
    assert probe.peak > 1  # proves the next test can fail


def test_wrapped_calls_never_overlap():
    probe = Probe()
    hammer(SerializedEmbedder(probe))
    assert probe.peak == 1


def test_identity_and_results_pass_through():
    inner = FakeEmbedder(16)
    wrapped = SerializedEmbedder(inner)
    assert (wrapped.model_id, wrapped.dim, wrapped.device) == (inner.model_id, 16, "cpu")
    np.testing.assert_array_equal(wrapped.embed_query("x"), inner.embed_query("x"))
    np.testing.assert_array_equal(
        wrapped.embed_documents(["x", "y"]), inner.embed_documents(["x", "y"])
    )
    assert wrapped.embed_images([]).shape == (0, 16)
```

Append to `sidecar/tests/test_fake_embedder.py`:

```python
def test_fake_embedder_reports_a_cpu_device():
    assert FakeEmbedder(8).device == "cpu"
```

(Check the file already imports `FakeEmbedder`; add the import if it does not.)

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_serialized_embedder.py tests/test_fake_embedder.py -q`
Expected: FAIL (`ModuleNotFoundError: omni_memory.embedding.serialized`; `AttributeError: device`).

- [ ] **Step 3: Dependencies**

In `sidecar/pyproject.toml` replace the `dependencies` block and the `dev` line with:

```toml
dependencies = [
    "fastapi",
    "numpy",
    "pathspec",
    "pillow",
    "pypdfium2",
    "pyyaml",
    "sentence-transformers[image]>=6.1.0",
    "threadpoolctl",
    "torch",
    "transformers",
    "uvicorn",
]

[dependency-groups]
dev = ["httpx2", "pytest", "ruff", "pyright"]
```

Run: `uv sync`
Expected: installs fastapi, uvicorn, httpx2 and their dependencies; `uv.lock` changes.

- [ ] **Step 4: Implement**

In `sidecar/src/omni_memory/embedding/base.py`, add this property to `Embedder` right after `dim`:

```python
    @property
    def device(self) -> str:
        """Where the model runs: `cuda` or `cpu`."""
        ...
```

In `sidecar/src/omni_memory/embedding/fake.py`, add after the `dim` property:

```python
    @property
    def device(self) -> str:
        return "cpu"
```

In `sidecar/src/omni_memory/eval/runner.py` replace `device = str(getattr(embedder, "device", "none"))` with `device = embedder.device`. In `sidecar/tests/test_eval.py` change the baseline written by `test_the_cli_fails_when_quality_drops_below_the_baseline` from `"device": "none"` to `"device": "cpu"`, and the comment in `test_a_run_on_another_device_is_not_gated_on_quality` from `device 'none' != 'cuda'` to `device 'cpu' != 'cuda'`.

Create `sidecar/src/omni_memory/embedding/serialized.py`:

```python
"""Lets several threads share one embedder.

The API serves search requests while an index job embeds documents, both on the same model.
Neither the GPU model nor its tokenizer is guaranteed re-entrant, so calls take turns. A
query therefore waits for at most the batch that is currently running (about 200 ms).
"""

from __future__ import annotations

import threading
from collections.abc import Sequence
from typing import TYPE_CHECKING

import numpy as np

from omni_memory.embedding.base import Embedder

if TYPE_CHECKING:
    from PIL.Image import Image


class SerializedEmbedder:
    def __init__(self, inner: Embedder) -> None:
        self._inner = inner
        self._lock = threading.Lock()

    @property
    def model_id(self) -> str:
        return self._inner.model_id

    @property
    def dim(self) -> int:
        return self._inner.dim

    @property
    def device(self) -> str:
        return self._inner.device

    def embed_documents(self, texts: Sequence[str]) -> np.ndarray:
        with self._lock:
            return self._inner.embed_documents(texts)

    def embed_images(self, images: Sequence[Image]) -> np.ndarray:
        with self._lock:
            return self._inner.embed_images(images)

    def embed_query(self, text: str) -> np.ndarray:
        with self._lock:
            return self._inner.embed_query(text)
```

Append to `sidecar/tests/helpers.py` (add `import time` and `from collections.abc import Callable` at the top):

```python
def wait_for[T](predicate: Callable[[], T], timeout: float = 10.0, interval: float = 0.01) -> T:
    """Poll until `predicate()` is truthy and return that value; fail loudly on timeout."""
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if value := predicate():
            return value
        time.sleep(interval)
    raise AssertionError(f"condition not met within {timeout} s")
```

- [ ] **Step 5: Run the gate**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass (4 new tests; the eval tests still pass with the `cpu` device).

- [ ] **Step 6: Commit**

```bash
git add sidecar/pyproject.toml sidecar/uv.lock sidecar/src sidecar/tests
git commit -m "feat(embedding): device on the Embedder contract and a thread-safe wrapper"
```

### Task 2: Background index jobs

**Files:**
- Create: `sidecar/src/omni_memory/jobs.py`, `sidecar/tests/api_helpers.py`
- Test: `sidecar/tests/test_jobs.py`

**Interfaces:**
- Consumes: `Indexer`, `SyncReport` (phase 3), `wait_for` (Task 1).
- Produces `omni_memory.jobs`:
  - `JobState(enum.StrEnum)`: `RUNNING = "running"`, `DONE = "done"`, `FAILED = "failed"`.
  - `Job` dataclass(slots): `id: str`, `source_id: str`, `root: str`, `state: JobState = RUNNING`, `files_done: int = 0`, `files_total: int = 0`, `report: SyncReport | None = None`, `error: str | None = None`.
  - `JobBusyError(job_id: str)` (an `Exception`; `.job_id`).
  - `JobManager(indexer: Indexer, *, keep: int = 50)`: `submit(source_id: str, root: Path) -> Job` (starts a daemon thread; raises `JobBusyError` if one is running; returns a snapshot), `get(job_id: str) -> Job | None` (snapshot), `running_source() -> str | None`. Job ids are `job-1`, `job-2`, ... A failing `Indexer.sync` (any `Exception`) ends the job as `FAILED` with `error = "<ExceptionType>: <message>"`; the manager stays usable. At most `keep` jobs are remembered; the oldest finished ones are dropped first.
- Produces test helpers in `tests/api_helpers.py`: `GatedEmbedder` (blocks inside `embed_documents` until `.release` is set; `.entered` is set when it first blocks), `FailingEmbedder` (raises `RuntimeError("gpu on fire")` in `embed_documents`), `SlowEmbedder` (sleeps 0.03 s per `embed_documents` call); all are `FakeEmbedder(64)` subclasses. (`running_app` and the constants are added to this file in Task 3.)

- [ ] **Step 1: Test helpers and failing tests**

`sidecar/tests/api_helpers.py`:

```python
"""Shared pieces for the job and HTTP API tests: embedders that misbehave on purpose."""

import threading
import time

from omni_memory.embedding import FakeEmbedder

DIM = 64


class GatedEmbedder(FakeEmbedder):
    """Holds an index job open: `embed_documents` blocks until `release` is set."""

    def __init__(self) -> None:
        super().__init__(DIM)
        self.entered = threading.Event()
        self.release = threading.Event()

    def embed_documents(self, texts):
        self.entered.set()
        assert self.release.wait(10), "the gate was never released"
        return super().embed_documents(texts)


class FailingEmbedder(FakeEmbedder):
    def __init__(self) -> None:
        super().__init__(DIM)

    def embed_documents(self, texts):
        raise RuntimeError("gpu on fire")


class SlowEmbedder(FakeEmbedder):
    """Makes each embedding call take a visible amount of time."""

    def __init__(self) -> None:
        super().__init__(DIM)

    def embed_documents(self, texts):
        time.sleep(0.03)
        return super().embed_documents(texts)
```

`sidecar/tests/test_jobs.py`:

```python
import pytest
from api_helpers import FailingEmbedder, GatedEmbedder
from helpers import wait_for

from omni_memory.config import Settings
from omni_memory.embedding import FakeEmbedder
from omni_memory.indexer import Indexer
from omni_memory.jobs import JobBusyError, JobManager, JobState
from omni_memory.store import FlatStore


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
```

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_jobs.py -q`
Expected: FAIL (`ModuleNotFoundError: omni_memory.jobs`).

- [ ] **Step 3: Implement `jobs.py`**

```python
"""Background indexing jobs: one at a time, with progress the API can poll.

A job wraps one `Indexer.sync` call in a daemon thread. Only one runs at a time because
indexing saturates the GPU and the store lock; a second request is refused with the id of the
running job so the caller can follow that one instead. Jobs live in memory only: the index
itself is the durable record, and re-running a sync is cheap.
"""

from __future__ import annotations

import dataclasses
import enum
import logging
import threading
from collections import OrderedDict
from dataclasses import dataclass
from pathlib import Path

from omni_memory.indexer import Indexer, SyncReport

logger = logging.getLogger(__name__)


class JobState(enum.StrEnum):
    RUNNING = "running"
    DONE = "done"
    FAILED = "failed"


@dataclass(slots=True)
class Job:
    id: str
    source_id: str
    root: str
    state: JobState = JobState.RUNNING
    files_done: int = 0
    files_total: int = 0
    report: SyncReport | None = None
    error: str | None = None


class JobBusyError(Exception):
    """Another job is running; `job_id` says which."""

    def __init__(self, job_id: str) -> None:
        super().__init__(f"{job_id} is still running")
        self.job_id = job_id


class JobManager:
    def __init__(self, indexer: Indexer, *, keep: int = 50) -> None:
        self._indexer = indexer
        self._keep = keep
        self._lock = threading.Lock()
        self._jobs: OrderedDict[str, Job] = OrderedDict()
        self._counter = 0

    def submit(self, source_id: str, root: Path) -> Job:
        """Start indexing `root` as `source_id`; returns a snapshot of the new job."""
        with self._lock:
            running = self._running()
            if running is not None:
                raise JobBusyError(running.id)
            self._counter += 1
            job = Job(id=f"job-{self._counter}", source_id=source_id, root=str(root))
            self._jobs[job.id] = job
            self._trim()
            snapshot = dataclasses.replace(job)
        threading.Thread(target=self._run, args=(job,), name=f"index-{job.id}", daemon=True).start()
        return snapshot

    def get(self, job_id: str) -> Job | None:
        with self._lock:
            job = self._jobs.get(job_id)
            return None if job is None else dataclasses.replace(job)

    def running_source(self) -> str | None:
        with self._lock:
            job = self._running()
            return None if job is None else job.source_id

    def _running(self) -> Job | None:
        return next((job for job in self._jobs.values() if job.state is JobState.RUNNING), None)

    def _trim(self) -> None:
        """Forget the oldest finished jobs beyond `keep` (never the running one)."""
        while len(self._jobs) > self._keep:
            oldest = next((i for i, j in self._jobs.items() if j.state is not JobState.RUNNING), None)
            if oldest is None:
                return
            del self._jobs[oldest]

    def _run(self, job: Job) -> None:
        def progress(done: int, total: int) -> None:
            with self._lock:
                job.files_done, job.files_total = done, total

        try:
            report: SyncReport = self._indexer.sync(job.source_id, Path(job.root), on_progress=progress)
        except Exception as error:  # the job fails; the service must stay up
            logger.exception("index job %s failed", job.id)
            with self._lock:
                job.state = JobState.FAILED
                job.error = f"{type(error).__name__}: {error}"
        else:
            with self._lock:
                job.report = report
                job.state = JobState.DONE
```

- [ ] **Step 4: Run the gate**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass (7 new tests).

- [ ] **Step 5: Commit**

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(jobs): one-at-a-time background index jobs with progress"
```

### Task 3: API core (app factory, auth, host check, `/health`)

**Files:**
- Create: `sidecar/src/omni_memory/api/__init__.py`, `deps.py`, `schemas.py`, `routes.py`, `app.py` (all under `sidecar/src/omni_memory/api/`)
- Modify: `sidecar/tests/api_helpers.py`, `sidecar/tests/conftest.py`
- Test: `sidecar/tests/test_api_core.py`

**Interfaces:**
- Consumes: `Settings`, `FlatStore`, `Searcher`, `Indexer`, `JobManager` (Task 2), `SerializedEmbedder` (Task 1), `Embedder`.
- Produces `omni_memory.api.deps`: `Engine` frozen dataclass (`embedder: Embedder`, `searcher: Searcher`, `jobs: JobManager`); `get_engine(request: Request) -> Engine` (raises `HTTPException(503)` while loading or after a failed load); `get_store(request: Request) -> FlatStore`; `token_guard(token: str) -> Callable[..., None]` (FastAPI dependency; 401 unless the `Authorization` header equals `Bearer <token>`, compared in constant time).
- Produces `omni_memory.api.schemas.HealthOut(ready: bool, model: str, device: str | None, embedded_items: int, version: str, error: str | None)`.
- Produces `omni_memory.api.routes.router` (`APIRouter`; Task 3 adds `GET /health`, Tasks 4-5 add the rest) and `omni_memory.api.app`: `MIN_TOKEN_CHARS = 16`, `ALLOWED_HOSTS = ["127.0.0.1", "localhost"]`, `create_app(settings: Settings, token: str, make_embedder: Callable[[], Embedder]) -> FastAPI`. The lifespan opens `FlatStore(settings.index_dir, model_id=settings.model_id, dim=settings.dim)`, starts a daemon thread that calls `make_embedder()` and builds the `Engine`, and closes the store on shutdown. `app.state` holds `settings`, `store`, `engine` (None until loaded) and `load_error` (None unless loading failed or the embedder's `model_id`/`dim` differ from `settings`).
- Produces test helpers: `api_helpers.TOKEN`, `AUTH`, `BASE`, `wait_ready(client)`, `index(client, folder, source_id="repo") -> dict` (POST /sources, wait for the job, return its JSON), `running_app(index_dir, make_embedder=<FakeEmbedder(64)>, *, wait=True, **settings_overrides)` (context manager yielding a `TestClient` with the auth header preset and the lifespan running), and `conftest` fixtures `api` (a ready app on `tmp_path / "idx"`) and `folder` (sample folder: `Backend/jwt.js`, `Backend/addcountries.js`, `notes.md`, plus a `.env`; under a directory name with a space).

- [ ] **Step 1: Test helpers**

Append to `sidecar/tests/api_helpers.py` (merge the new imports into the existing import block):

```python
import contextlib
from collections.abc import Callable, Iterator
from pathlib import Path

from fastapi.testclient import TestClient
from helpers import wait_for

from omni_memory.api.app import create_app
from omni_memory.config import Settings
from omni_memory.embedding.base import Embedder

TOKEN = "t0ken-" + "x" * 26
AUTH = {"Authorization": f"Bearer {TOKEN}"}
BASE = "http://127.0.0.1:7861"


def wait_ready(client: TestClient) -> None:
    def check() -> bool:
        body = client.get("/health").json()
        assert body["error"] is None, body["error"]
        return body["ready"]

    wait_for(check)


def index(client: TestClient, folder: Path, source_id: str = "repo") -> dict:
    """Add a folder and wait for its job to finish; returns the finished job as JSON."""
    response = client.post("/sources", json={"path": str(folder), "source_id": source_id})
    assert response.status_code == 202, response.text
    job_id = response.json()["job_id"]
    return wait_for(lambda: (j := client.get(f"/jobs/{job_id}").json())["state"] != "running" and j)


@contextlib.contextmanager
def running_app(
    index_dir: Path,
    make_embedder: Callable[[], Embedder] = lambda: FakeEmbedder(DIM),
    *,
    wait: bool = True,
    **settings_overrides,
) -> Iterator[TestClient]:
    """A live app (lifespan running) with the token preset on the client."""
    probe = FakeEmbedder(DIM)
    settings = Settings(
        index_dir=index_dir, model_id=probe.model_id, dim=DIM, **settings_overrides
    )
    app = create_app(settings, TOKEN, make_embedder)
    with TestClient(app, base_url=BASE, headers=AUTH) as client:
        if wait:
            wait_ready(client)
        yield client
```

Append to `sidecar/tests/conftest.py` (add `from api_helpers import running_app` to its imports):

```python
@pytest.fixture
def api(tmp_path):
    with running_app(tmp_path / "idx") as client:
        yield client


@pytest.fixture
def folder(tmp_path):
    root = tmp_path / "docs folder"  # a space on purpose
    (root / "Backend").mkdir(parents=True)
    (root / "Backend" / "jwt.js").write_text("function signRefreshToken() { refresh token jwt }")
    (root / "Backend" / "addcountries.js").write_text("async function addCountries() { seed }")
    (root / "notes.md").write_text("# Notes\nthe quick brown fox jumps over the lazy dog")
    (root / ".env").write_text("JWT_SECRET=hunter2")
    return root
```

- [ ] **Step 2: Write the failing tests**

`sidecar/tests/test_api_core.py`:

```python
import threading

import pytest
from api_helpers import BASE, DIM, TOKEN, running_app, wait_ready
from fastapi.testclient import TestClient
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
        assert client.get("/jobs/job-1").status_code == 503


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
```

- [ ] **Step 3: Run to see them fail**

Run: `uv run pytest tests/test_api_core.py -q`
Expected: FAIL (`ModuleNotFoundError: omni_memory.api`).

- [ ] **Step 4: Implement the package**

`sidecar/src/omni_memory/api/__init__.py`:

```python
"""The HTTP API: authenticated, localhost only, a thin layer over the engine."""
```

`sidecar/src/omni_memory/api/deps.py`:

```python
"""Request-scoped dependencies: the loaded engine, the store and the token check."""

from __future__ import annotations

import hmac
from collections.abc import Callable
from dataclasses import dataclass
from typing import Annotated

from fastapi import Header, HTTPException, Request

from omni_memory.embedding.base import Embedder
from omni_memory.jobs import JobManager
from omni_memory.search.searcher import Searcher
from omni_memory.store import FlatStore


@dataclass(frozen=True, slots=True)
class Engine:
    """Everything that needs the embedding model; exists only once the model has loaded."""

    embedder: Embedder
    searcher: Searcher
    jobs: JobManager


def get_engine(request: Request) -> Engine:
    state = request.app.state
    if state.engine is None:
        reason = "the model is still loading"
        if state.load_error is not None:
            reason = f"the model failed to load: {state.load_error}"
        raise HTTPException(status_code=503, detail=reason)
    return state.engine


def get_store(request: Request) -> FlatStore:
    return request.app.state.store


def token_guard(token: str) -> Callable[..., None]:
    """A dependency that admits only `Authorization: Bearer <token>` (constant-time compare)."""
    expected = f"Bearer {token}".encode()

    def guard(authorization: Annotated[str | None, Header()] = None) -> None:
        if authorization is None or not hmac.compare_digest(authorization.encode(), expected):
            raise HTTPException(
                status_code=401,
                detail="missing or wrong bearer token",
                headers={"WWW-Authenticate": "Bearer"},
            )

    return guard
```

`sidecar/src/omni_memory/api/schemas.py`:

```python
"""Request and response bodies. Unknown request fields are errors, not silently ignored."""

from __future__ import annotations

from pydantic import BaseModel


class HealthOut(BaseModel):
    ready: bool
    model: str
    device: str | None
    embedded_items: int
    version: str
    error: str | None
```

`sidecar/src/omni_memory/api/routes.py`:

```python
"""The API routes. Auth and host checks are applied around them in `create_app`."""

from __future__ import annotations

from importlib.metadata import version

from fastapi import APIRouter, Request

from omni_memory.api.schemas import HealthOut

router = APIRouter()

_VERSION = version("omni-sidecar")


@router.get("/health", response_model=HealthOut)
def health(request: Request) -> HealthOut:
    state = request.app.state
    engine = state.engine
    return HealthOut(
        ready=engine is not None,
        model=state.settings.model_id,
        device=None if engine is None else engine.embedder.device,
        embedded_items=state.store.count(),
        version=_VERSION,
        error=state.load_error,
    )
```

`sidecar/src/omni_memory/api/app.py`:

```python
"""Application factory: wires the store, the background model load, auth and the host check.

The model takes ~10 s to load. The store opens at once and the model loads on a daemon thread,
so the port answers `/health` immediately (`ready: false`) and the launcher can show progress
instead of waiting on a closed port. Every other route answers 503 until the engine exists.
"""

from __future__ import annotations

import contextlib
import logging
import threading
from collections.abc import AsyncIterator, Callable

from fastapi import Depends, FastAPI
from starlette.middleware.trustedhost import TrustedHostMiddleware

from omni_memory.api.deps import Engine, token_guard
from omni_memory.api.routes import router
from omni_memory.config import Settings
from omni_memory.embedding.base import Embedder
from omni_memory.embedding.serialized import SerializedEmbedder
from omni_memory.indexer import Indexer
from omni_memory.jobs import JobManager
from omni_memory.search.searcher import Searcher
from omni_memory.store import FlatStore

logger = logging.getLogger(__name__)

MIN_TOKEN_CHARS = 16
# Anything else (a web page resolving its own name to 127.0.0.1) is a DNS-rebinding attempt.
ALLOWED_HOSTS = ["127.0.0.1", "localhost"]


def _load_engine(
    app: FastAPI, store: FlatStore, settings: Settings, make_embedder: Callable[[], Embedder]
) -> None:
    try:
        embedder = SerializedEmbedder(make_embedder())
        if (embedder.model_id, embedder.dim) != (settings.model_id, settings.dim):
            raise RuntimeError(
                f"embedder is {embedder.model_id} dim {embedder.dim} but the index is "
                f"{settings.model_id} dim {settings.dim}"
            )
        app.state.engine = Engine(
            embedder=embedder,
            searcher=Searcher(store, embedder),
            jobs=JobManager(Indexer(store, embedder, settings)),
        )
    except Exception as error:  # report through /health instead of killing the process
        logger.exception("loading the embedding model failed")
        app.state.load_error = f"{type(error).__name__}: {error}"


def create_app(
    settings: Settings, token: str, make_embedder: Callable[[], Embedder]
) -> FastAPI:
    if len(token) < MIN_TOKEN_CHARS:
        raise ValueError(f"the API token must have at least {MIN_TOKEN_CHARS} characters")

    @contextlib.asynccontextmanager
    async def lifespan(app: FastAPI) -> AsyncIterator[None]:
        store = FlatStore(settings.index_dir, model_id=settings.model_id, dim=settings.dim)
        app.state.store = store
        threading.Thread(
            target=_load_engine,
            args=(app, store, settings, make_embedder),
            name="model-loader",
            daemon=True,
        ).start()
        try:
            yield
        finally:
            store.close()

    app = FastAPI(title="Omni memory engine", lifespan=lifespan, docs_url=None, redoc_url=None, openapi_url=None)
    app.state.settings = settings
    app.state.engine = None
    app.state.load_error = None
    app.include_router(router, dependencies=[Depends(token_guard(token))])
    # Added last, so it runs first: a foreign Host is refused before the token is even read.
    app.add_middleware(TrustedHostMiddleware, allowed_hosts=ALLOWED_HOSTS)
    return app
```

- [ ] **Step 5: Run the gate**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass (about 16 new test cases). If the model-load-failure tests spin forever, the app is not setting `load_error`; do not add sleeps, fix the loader.

- [ ] **Step 6: Commit**

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(api): app factory with bearer auth, host check, background model load and /health"
```

### Task 4: Sources, jobs and delete routes

**Files:**
- Modify: `sidecar/src/omni_memory/api/schemas.py`, `sidecar/src/omni_memory/api/routes.py`
- Test: `sidecar/tests/test_api_sources.py`

**Interfaces:**
- Consumes: `Engine`, `get_engine`, `get_store` (Task 3), `JobManager`, `JobBusyError`, `Job`, `SyncReport`, `FlatStore.delete_source`.
- Produces `schemas`: `SOURCE_ID_PATTERN = r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$"`; `AddSource(path: str, source_id: str)` (extra fields forbidden, `path` 1-1024 chars); `SourceAccepted(job_id: str)`; `FileErrorOut(uri, message)`; `ReportOut(added, updated, unchanged, removed, denied, ignored, unsupported, too_large, chunks_written: int; errors: list[FileErrorOut])`; `JobOut(id, source_id, state: JobState, files_done, files_total: int, report: ReportOut | None, error: str | None)`; `RemovedOut(removed_chunks: int)`.
- Produces routes: `POST /sources` (202 `SourceAccepted`; 400 if `path` is not an absolute path to an existing folder; 409 `{"detail": {"message", "job_id"}}` if a job is running; 422 on bad bodies; 503 while loading), `GET /jobs/{job_id}` (200 `JobOut`, 404 unknown, 503 while loading), `DELETE /sources/{source_id}` (200 `RemovedOut`, idempotent; 409 if that source is being indexed; 422 for an invalid id; 503 while loading).

- [ ] **Step 1: Write the failing tests**

`sidecar/tests/test_api_sources.py`:

```python
import threading

import pytest
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


def test_a_file_is_not_a_folder(api, folder):
    response = api.post("/sources", json={"path": str(folder / "notes.md"), "source_id": "repo"})
    assert response.status_code == 400


@pytest.mark.parametrize("source_id", ["", "../x", "a b", "-lead", ".hidden", "x" * 65, "a/b"])
def test_invalid_source_ids_are_rejected(api, folder, source_id):
    response = api.post("/sources", json={"path": str(folder), "source_id": source_id})
    assert response.status_code == 422


@pytest.mark.parametrize(
    "body",
    [{}, {"path": "C:/x"}, {"source_id": "r"}, {"path": 5, "source_id": "r"}, {"path": "C:/x", "source_id": "r", "extra": 1}],
)
def test_malformed_bodies_are_rejected(api, body):
    assert api.post("/sources", json=body).status_code == 422


def test_a_non_json_body_is_rejected(api):
    response = api.post("/sources", content=b"not json", headers={"Content-Type": "application/json"})
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
        job_id = api.post("/sources", json={"path": str(folder), "source_id": "repo"}).json()["job_id"]
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


def test_secret_files_in_an_added_folder_are_counted_not_indexed(api, folder):
    job = index(api, folder)
    assert job["report"]["denied"] == 1
    assert job["report"]["added"] == 3  # the .env file is not among the indexed files
```

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_api_sources.py -q`
Expected: FAIL (404 for `/sources`, `/jobs/...`).

- [ ] **Step 3: Implement schemas and routes**

Replace `sidecar/src/omni_memory/api/schemas.py` with:

```python
"""Request and response bodies. Unknown request fields are errors, not silently ignored."""

from __future__ import annotations

from pydantic import BaseModel, ConfigDict, Field

from omni_memory.jobs import JobState

# Also validated for the path parameter of DELETE /sources/{source_id}.
SOURCE_ID_PATTERN = r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$"


class _Request(BaseModel):
    model_config = ConfigDict(extra="forbid")


class _Response(BaseModel):
    model_config = ConfigDict(from_attributes=True)


class HealthOut(BaseModel):
    ready: bool
    model: str
    device: str | None
    embedded_items: int
    version: str
    error: str | None


class AddSource(_Request):
    path: str = Field(min_length=1, max_length=1024)
    source_id: str = Field(pattern=SOURCE_ID_PATTERN)


class SourceAccepted(BaseModel):
    job_id: str


class FileErrorOut(_Response):
    uri: str
    message: str


class ReportOut(_Response):
    added: int
    updated: int
    unchanged: int
    removed: int
    denied: int
    ignored: int
    unsupported: int
    too_large: int
    chunks_written: int
    errors: list[FileErrorOut]


class JobOut(_Response):
    id: str
    source_id: str
    state: JobState
    files_done: int
    files_total: int
    report: ReportOut | None
    error: str | None


class RemovedOut(BaseModel):
    removed_chunks: int
```

Replace `sidecar/src/omni_memory/api/routes.py` with:

```python
"""The API routes. Auth and host checks are applied around them in `create_app`."""

from __future__ import annotations

from importlib.metadata import version
from pathlib import Path
from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Request
from fastapi import Path as PathParam

from omni_memory.api.deps import Engine, get_engine, get_store
from omni_memory.api.schemas import (
    SOURCE_ID_PATTERN,
    AddSource,
    HealthOut,
    JobOut,
    RemovedOut,
    SourceAccepted,
)
from omni_memory.jobs import Job, JobBusyError
from omni_memory.store import FlatStore

router = APIRouter()

_VERSION = version("omni-sidecar")


@router.get("/health", response_model=HealthOut)
def health(request: Request) -> HealthOut:
    state = request.app.state
    engine = state.engine
    return HealthOut(
        ready=engine is not None,
        model=state.settings.model_id,
        device=None if engine is None else engine.embedder.device,
        embedded_items=state.store.count(),
        version=_VERSION,
        error=state.load_error,
    )


@router.post("/sources", status_code=202)
def add_source(body: AddSource, engine: Annotated[Engine, Depends(get_engine)]) -> SourceAccepted:
    root = Path(body.path)
    if not root.is_absolute() or not root.is_dir():
        raise HTTPException(status_code=400, detail="path must be an absolute path to an existing folder")
    try:
        job = engine.jobs.submit(body.source_id, root)
    except JobBusyError as busy:
        raise HTTPException(
            status_code=409, detail={"message": str(busy), "job_id": busy.job_id}
        ) from busy
    return SourceAccepted(job_id=job.id)


@router.get("/jobs/{job_id}", response_model=JobOut)
def get_job(job_id: str, engine: Annotated[Engine, Depends(get_engine)]) -> Job:
    job = engine.jobs.get(job_id)
    if job is None:
        raise HTTPException(status_code=404, detail=f"no such job: {job_id}")
    return job


@router.delete("/sources/{source_id}")
def delete_source(
    source_id: Annotated[str, PathParam(pattern=SOURCE_ID_PATTERN)],
    engine: Annotated[Engine, Depends(get_engine)],
    store: Annotated[FlatStore, Depends(get_store)],
) -> RemovedOut:
    if engine.jobs.running_source() == source_id:
        raise HTTPException(status_code=409, detail=f"{source_id} is being indexed")
    return RemovedOut(removed_chunks=store.delete_source(source_id))
```

- [ ] **Step 4: Run the gate**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass (about 30 new test cases). pyright may want a return annotation on `get_job`; use `-> Job` imported from `omni_memory.jobs` (FastAPI converts it through `response_model`), not `Any`.

- [ ] **Step 5: Commit**

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(api): add and delete sources, poll index jobs"
```

### Task 5: Search route and the cross-cutting auth test

**Files:**
- Modify: `sidecar/src/omni_memory/api/schemas.py`, `sidecar/src/omni_memory/api/routes.py`
- Test: `sidecar/tests/test_api_search.py`, `sidecar/tests/test_api_auth.py`

**Interfaces:**
- Consumes: `Searcher.search(query, k, filters)`, `SearchHit`, `Filters`, `ChunkKind`, `Engine` (Tasks 3-4).
- Produces `schemas`: `SearchFilters(kinds: list[ChunkKind] | None, source_id: str | None (SOURCE_ID_PATTERN), uri_prefix: str | None (max 1024), modified_after_ns: int | None (>= 0))` (extra forbidden); `SearchRequest(query: str (max 4000, may be empty), k: int = 10 (1-100), filters: SearchFilters | None)` (extra forbidden); `SearchHitOut(id, uri, kind: ChunkKind, locator, score: float, vector_score: float | None, keyword_rank: int | None, snippet)`; `SearchResponse(hits: list[SearchHitOut])`.
- Produces route `POST /search` (200 `SearchResponse`; an empty or blank query returns `{"hits": []}`; 422 for invalid bodies; 503 while loading).

- [ ] **Step 1: Write the failing tests**

`sidecar/tests/test_api_search.py`:

```python
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


def test_an_exact_identifier_is_found_by_its_name(indexed):
    assert search(indexed, "addCountries", k=1).json()["hits"][0]["uri"] == "Backend/addcountries.js"


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
    assert search(indexed, "token", filters={"source_id": "nope"}).json() == {"hits": []}
    assert search(indexed, "token", filters={"modified_after_ns": 2**62}).json() == {"hits": []}
    assert search(indexed, "token", filters={"source_id": "repo"}).json()["hits"]


def test_secret_files_are_not_searchable_through_the_api(indexed):
    for query in ("hunter2", "JWT_SECRET"):
        uris = [h["uri"] for h in search(indexed, query).json()["hits"]]
        assert not [u for u in uris if ".env" in u]


@pytest.mark.parametrize(
    "query",
    ["", "   ", "!!! ???", "the of and", 'say "hi" * NEAR( x ) OR AND NOT', "x" * 4000, "token " * 600],
)
def test_odd_queries_return_a_list_not_an_error(indexed, query):
    response = search(indexed, query)
    assert response.status_code == 200
    assert isinstance(response.json()["hits"], list)


@pytest.mark.parametrize("query", ["", "   "])
def test_blank_queries_have_no_hits(indexed, query):
    assert search(indexed, query).json() == {"hits": []}


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
    assert search(api, "anything").json() == {"hits": []}


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
```

`sidecar/tests/test_api_auth.py`:

```python
import pytest
from api_helpers import BASE, TOKEN
from fastapi.testclient import TestClient

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
    response = bare.request(method, path, json={"query": 5, "bogus": 1}, headers={"Authorization": "Bearer wrong"})
    assert response.status_code == 401


@pytest.mark.parametrize(("method", "path", "body"), ROUTES)
def test_a_foreign_host_is_rejected_on_every_route_even_with_the_token(api, method, path, body):
    response = api.request(method, path, json=body, headers={"Host": "evil.example"})
    assert response.status_code == 400
```

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_api_search.py tests/test_api_auth.py -q`
Expected: search tests FAIL (404 for `/search`); the auth tests for `/search` fail the same way (404 vs 401 is impossible before the route exists, so `test_every_route_rejects_a_bad_token[...-/search...]` shows 404 or 401 depending on routing; read the failures and confirm they are about the missing route only).

- [ ] **Step 3: Implement**

Append to `sidecar/src/omni_memory/api/schemas.py` (add `from omni_memory.types import ChunkKind` to the imports):

```python
class SearchFilters(_Request):
    kinds: list[ChunkKind] | None = None
    source_id: str | None = Field(default=None, pattern=SOURCE_ID_PATTERN)
    uri_prefix: str | None = Field(default=None, max_length=1024)
    modified_after_ns: int | None = Field(default=None, ge=0)


class SearchRequest(_Request):
    query: str = Field(max_length=4000)  # empty is allowed and returns no hits
    k: int = Field(default=10, ge=1, le=100)
    filters: SearchFilters | None = None


class SearchHitOut(_Response):
    id: str
    uri: str
    kind: ChunkKind
    locator: str
    score: float
    vector_score: float | None
    keyword_rank: int | None
    snippet: str


class SearchResponse(BaseModel):
    hits: list[SearchHitOut]
```

In `sidecar/src/omni_memory/api/routes.py` extend the schema import with `SearchRequest, SearchResponse, SearchHitOut`, import `Filters` from `omni_memory.types`, and append:

```python
@router.post("/search")
def search(body: SearchRequest, engine: Annotated[Engine, Depends(get_engine)]) -> SearchResponse:
    filters = None
    if body.filters is not None:
        filters = Filters(
            kinds=None if body.filters.kinds is None else frozenset(body.filters.kinds),
            source_id=body.filters.source_id,
            uri_prefix=body.filters.uri_prefix,
            modified_after_ns=body.filters.modified_after_ns,
        )
    hits = engine.searcher.search(body.query, body.k, filters)
    return SearchResponse(hits=[SearchHitOut.model_validate(hit) for hit in hits])
```

- [ ] **Step 4: Run the gate**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass (about 90 new test cases).

- [ ] **Step 5: Commit**

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(api): hybrid search route; test authentication on every route"
```

### Task 6: Process entry point and the start/ready/stop contract

**Files:**
- Create: `sidecar/src/omni_memory/__main__.py`
- Test: `sidecar/tests/test_main.py`, `sidecar/tests/test_api_process.py`

**Interfaces:**
- Consumes: `create_app`, `MIN_TOKEN_CHARS` (Task 3), `Settings.from_env`, `ConfigError`, `GemmaEmbedder`.
- Produces `python -m omni_memory`: reads `OMNI_SIDECAR_TOKEN` (required, >= 16 chars), `OMNI_SIDECAR_PORT` (optional, 1-65535, default 7861), `OMNI_MEMORY_DIR`/`LOCALAPPDATA` (index location, as in `Settings.from_env`); exit status 2 with a one-line message on stderr for any of those being wrong (checked before anything heavy is imported); otherwise serves on `127.0.0.1` until terminated. `main() -> int`.

- [ ] **Step 1: Write the failing tests**

`sidecar/tests/test_main.py`:

```python
import os
import subprocess
import sys

import pytest


def run_main(tmp_path, **env_overrides):
    env = {k: v for k, v in os.environ.items() if not k.startswith("OMNI_")}
    env["OMNI_MEMORY_DIR"] = str(tmp_path / "idx")
    env.update(env_overrides)
    return subprocess.run(
        [sys.executable, "-m", "omni_memory"], env=env, capture_output=True, text=True, timeout=60
    )


def test_it_refuses_to_start_without_a_token(tmp_path):
    result = run_main(tmp_path)
    assert result.returncode == 2
    assert "OMNI_SIDECAR_TOKEN" in result.stderr


def test_it_refuses_a_short_token(tmp_path):
    result = run_main(tmp_path, OMNI_SIDECAR_TOKEN="short")
    assert result.returncode == 2
    assert "OMNI_SIDECAR_TOKEN" in result.stderr


@pytest.mark.parametrize("port", ["abc", "0", "70000", "-5", ""])
def test_it_refuses_a_bad_port(tmp_path, port):
    result = run_main(tmp_path, OMNI_SIDECAR_TOKEN="t" * 32, OMNI_SIDECAR_PORT=port)
    assert result.returncode == 2
    assert "OMNI_SIDECAR_PORT" in result.stderr


def test_it_refuses_to_guess_where_the_index_lives(tmp_path):
    env = {k: v for k, v in os.environ.items() if not k.startswith("OMNI_") and k != "LOCALAPPDATA"}
    env["OMNI_SIDECAR_TOKEN"] = "t" * 32
    result = subprocess.run(
        [sys.executable, "-m", "omni_memory"], env=env, capture_output=True, text=True, timeout=60
    )
    assert result.returncode == 2
    assert "OMNI_MEMORY_DIR" in result.stderr
```

`sidecar/tests/test_api_process.py`:

```python
import json
import os
import socket
import subprocess
import sys
import urllib.error
import urllib.request

import pytest
from helpers import wait_for

from omni_memory.config import EMBEDDING_DIM, MODEL_ID
from omni_memory.store import FlatStore

pytestmark = pytest.mark.model

TOKEN = "p" * 32


def free_port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def call(port, method, path, *, token=TOKEN, body=None):
    request = urllib.request.Request(
        f"http://127.0.0.1:{port}{path}",
        method=method,
        data=None if body is None else json.dumps(body).encode(),
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            return response.status, json.loads(response.read())
    except urllib.error.HTTPError as error:
        return error.code, json.loads(error.read())


def answering(port):
    try:
        return call(port, "GET", "/health")[1]
    except (urllib.error.URLError, ConnectionError, TimeoutError):
        return None


def test_the_real_process_honours_the_start_ready_stop_contract(tmp_path):
    folder = tmp_path / "project"
    (folder / "Backend").mkdir(parents=True)
    (folder / "Backend" / "jwt.js").write_text("function signRefreshToken() { refresh token jwt }")
    (folder / "Backend" / "addcountries.js").write_text("async function addCountries() { seed }")
    (folder / "notes.md").write_text("# Notes\nthe quick brown fox jumps over the lazy dog")
    port = free_port()
    env = {
        **os.environ,
        "OMNI_SIDECAR_TOKEN": TOKEN,
        "OMNI_SIDECAR_PORT": str(port),
        "OMNI_MEMORY_DIR": str(tmp_path / "idx"),
    }
    process = subprocess.Popen(
        [sys.executable, "-m", "omni_memory"], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
    )
    try:
        wait_for(lambda: answering(port), timeout=60)  # the port opens before the model is loaded
        health = wait_for(lambda: (h := answering(port)) and h["ready"] and h, timeout=180)
        assert (health["model"], health["error"]) == (MODEL_ID, None)
        assert health["device"] in {"cuda", "cpu"}
        assert call(port, "GET", "/health", token="wrong")[0] == 401

        status, accepted = call(
            port, "POST", "/sources", body={"path": str(folder), "source_id": "project"}
        )
        assert status == 202
        job = wait_for(
            lambda: (j := call(port, "GET", f"/jobs/{accepted['job_id']}")[1])["state"] != "running" and j,
            timeout=180,
        )
        assert (job["state"], job["report"]["added"]) == ("done", 3)

        status, found = call(
            port, "POST", "/search", body={"query": "how do I refresh the jwt token", "k": 3}
        )
        assert status == 200
        assert found["hits"][0]["uri"] == "Backend/jwt.js"
    finally:
        process.terminate()  # what the Go side does (a Job Object kills it the same way)
        process.wait(timeout=30)
    store = FlatStore(tmp_path / "idx", model_id=MODEL_ID, dim=EMBEDDING_DIM)
    try:
        assert store.count() == 3  # the index survived a hard stop
    finally:
        store.close()
```

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_main.py -q`
Expected: FAIL (`No module named omni_memory.__main__`).

- [ ] **Step 3: Implement `__main__.py`**

```python
"""Process entry point: `uv run python -m omni_memory`.

Contract with the launcher (docs/api.md): the token arrives in `OMNI_SIDECAR_TOKEN`, the port in
`OMNI_SIDECAR_PORT`; the process binds 127.0.0.1 only, opens the port within a second, reports
`ready: false` on `/health` while the model loads, and is stopped by terminating it.
"""

from __future__ import annotations

import os
import sys

from omni_memory.api.app import MIN_TOKEN_CHARS, create_app
from omni_memory.config import ConfigError, Settings
from omni_memory.embedding.base import Embedder

DEFAULT_PORT = 7861


def _fail(message: str) -> int:
    print(f"omni_memory: {message}", file=sys.stderr)
    return 2


def _load_gemma(settings: Settings) -> Embedder:
    # Imported here so a bad configuration fails fast, without importing torch first.
    from omni_memory.embedding.gemma import GemmaEmbedder

    return GemmaEmbedder(settings)


def main() -> int:
    token = os.environ.get("OMNI_SIDECAR_TOKEN", "")
    if len(token) < MIN_TOKEN_CHARS:
        return _fail(f"OMNI_SIDECAR_TOKEN must be set to at least {MIN_TOKEN_CHARS} characters")
    port_text = os.environ.get("OMNI_SIDECAR_PORT", str(DEFAULT_PORT))
    if not port_text.isdecimal() or not 1 <= int(port_text) <= 65535:
        return _fail(f"OMNI_SIDECAR_PORT must be a port number (1-65535), got {port_text!r}")
    try:
        settings = Settings.from_env()
    except ConfigError as error:
        return _fail(str(error))

    import uvicorn

    app = create_app(settings, token, lambda: _load_gemma(settings))
    uvicorn.run(app, host="127.0.0.1", port=int(port_text), log_level="info")
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Run the tests, then the gate**

Run: `uv run pytest tests/test_main.py -v`
Expected: 9 passed (each subprocess exits in about a second; if one takes 10+ s, something imports torch before the checks).
Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass.
Run: `uv run pytest -m model -q`
Expected: the earlier real-model tests plus `test_the_real_process_honours_the_start_ready_stop_contract` pass (about 1-2 minutes). If the process test fails, run it with `-s` and read the process's stderr by temporarily removing `stderr=subprocess.DEVNULL`; fix the cause, do not loosen the test.

- [ ] **Step 5: Commit**

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(api): python -m omni_memory entry point and a real-process contract test"
```

### Task 7: Documentation and the phase 5 gate

**Files:**
- Create: `docs/api.md`
- Modify: `docs/ARCHITECTURE.md`, `docs/STATUS.md`, `docs/README.md`, `AGENTS.md`

**Interfaces:** none (documentation).

- [ ] **Step 1: Write `docs/api.md`**

Create `docs/api.md` with these sections (write prose in the same style as the other docs; keep it exact, a Go developer will code against it):

1. **Process contract**: start `uv run python -m omni_memory` from `sidecar/` with env `OMNI_SIDECAR_TOKEN` (>= 16 characters, generated by Go per launch), `OMNI_SIDECAR_PORT` (default 7861), `OMNI_MEMORY_DIR` (optional; default `%LOCALAPPDATA%\Omni\memory`). Exit status 2 and a one-line stderr message for bad configuration. The port opens within about a second; poll `GET /health` until `ready` is true (model load 10-15 s warm); if `error` is non-null the model failed to load and `ready` will never become true. Stop by terminating the process (attach it to a Windows Job Object so it cannot outlive Go); the index survives a hard stop.
2. **Security**: bind `127.0.0.1` only; every route needs `Authorization: Bearer <token>`; `Host` must be `127.0.0.1` or `localhost` (use `127.0.0.1`; `[::1]` is rejected); wrong host gives 400, wrong or missing token 401 (and 401 wins over 422); no CORS, no docs pages.
3. **Routes**, one subsection each with request body, response body and every status code, exactly as built: `GET /health`, `POST /sources`, `GET /jobs/{job_id}`, `DELETE /sources/{source_id}`, `POST /search` (include the hit fields and the filters object; `modified_after_ns` is integer nanoseconds since the epoch; `k` is 1-100; `query` up to 4000 characters, empty returns no hits; raw scores are not calibrated, there is no "no good answer" signal yet).
4. **Semantics worth knowing**: one job at a time (409 carries the running `job_id`); posting an already-indexed folder again is an incremental sync (the way to refresh); job history is in memory (last 50) and lost on restart; `DELETE` is idempotent; `source_id` pattern `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`; paths must be absolute and exist; secrets are never indexed (deny list, counted in `report.denied`); a search issued while indexing waits at most one embedding batch.
5. **Deviations from the spec** (the list in this plan's File Structure section).

- [ ] **Step 2: Update the other docs**

`docs/ARCHITECTURE.md`: set the `api/` row to `built` with the job "FastAPI app: bearer auth, host check, background model load, `/health` `/sources` `/jobs` `/search`; see `api.md`"; change the `jobs.py` row to `built` ("`JobManager`: one background index job at a time, with progress"); add a row `embedding/serialized.py` ("`SerializedEmbedder`: lets the job thread and request threads share one model") or extend the `embedding/` row; update the data-flow paragraph to mention the API; add Invariants: "8. **The API is local and authenticated.** It binds 127.0.0.1, rejects foreign `Host` headers before anything else, and refuses to start without a token of at least 16 characters; every route including `/health` needs the token." and "9. **The API never blocks on the model.** The port opens immediately; `/health` reports `ready: false` while the model loads and every other route answers 503."
`docs/STATUS.md`: phase 5 `done`, update the date, set "Next" to "Phase 6 (quality iteration, driven by the weak groups in the baseline report) and the Go launcher that consumes `docs/api.md`", add the deviations to "Known gaps" (health `error`, idempotent delete, `device` on `Embedder`, IPv6 host rejected).
`docs/README.md`: add a row `api.md` | "The HTTP contract the Go launcher codes against: process contract, routes, status codes."
`AGENTS.md`: under `## Commits` add the line: "No AI attribution: never add `Co-Authored-By` or 'Generated with' trailers to commits or PR descriptions."

- [ ] **Step 3: Final gate and commit**

Run: `uv run ruff check . && uv run ruff format --check . && uv run pyright && uv run pytest -q` (all clean), `uv run pytest -m model -q` (real-model tests including the process test pass), `uv run python -m omni_memory.eval` (exit 0: retrieval quality unchanged), and `git status --short` (nothing under `Sample Data/` or any `.env`).

```bash
git add docs AGENTS.md
git commit -m "docs: HTTP API contract and phase 5 status"
```

---

## Self-Review (done against the spec)

**Spec coverage (section 8 and phase 5).** `GET /health` with `{ready, model, device, embedded_items, version}` and `ready` only after the model loads (T3); `POST /sources` returning a `job_id` (T4); `GET /jobs/{id}` with progress, counts and per-file errors (T2, T4); `DELETE /sources/{source_id}` (T4); `POST /search` with `{query, k?, filters?}` and the section 7 hit shape including raw scores and no "no good answer" signal (T5); `127.0.0.1` binding, configurable port default 7861, bearer token from `OMNI_SIDECAR_TOKEN`, non-local `Host` rejected, refusal to start without a token (T3, T6); "one job at a time" (T2); process contract `uv run python -m omni_memory`, poll `/health`, stop by terminating (T6, T7); "start/ready/stop contract documented" (T7). Deferred by design: the Go side of the Job Object, `Reranker` hook and everything in phase 6.

**Placeholders.** None.

**Type consistency.** `Engine(embedder, searcher, jobs)` is built in `app._load_engine` and read in `deps`/`routes` with the same names; `JobManager.submit/get/running_source` and `JobBusyError.job_id` match their uses in `routes`; `SOURCE_ID_PATTERN` is shared by `AddSource`, `SearchFilters` and the DELETE path parameter; `JobOut` reads `Job` attributes (`id, source_id, state, files_done, files_total, report, error`) and `ReportOut` reads `SyncReport` attributes (`added ... chunks_written, errors`), all of which exist; `running_app`, `TOKEN`, `AUTH`, `BASE`, `wait_ready`, `GatedEmbedder`, `FailingEmbedder`, `SlowEmbedder`, `index` are defined before the tasks that import them (`index` lives in `test_api_sources.py` and is imported by `test_api_search.py`).

**Known soft spots to watch while executing.** Starlette's `TrustedHostMiddleware` splits the `Host` header on `:` and so cannot match `[::1]`; that is accepted and documented. T3's load-failure tests poll `/health` until `error` appears: they hang if the loader thread never reports, which is the bug to fix, not to paper over. T5's concurrency test needs `flush_chunks=1` so the slow embedder is called many times (one long call would hold the embedder lock for the whole job and prove nothing). `httpx2` is the import the installed Starlette `TestClient` expects (plain `httpx` works but warns); if a different test-client package is required by the locked Starlette, ledger the ruling.
