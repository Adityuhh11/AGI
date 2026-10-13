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
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

from agi_memory.indexer import Indexer, SyncReport

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

    def run_unless_indexing[T](self, source_id: str, action: Callable[[], T]) -> T:
        """Run `action` only if no job is indexing `source_id`, and keep new jobs out meanwhile."""
        with self._lock:
            running = self._running()
            if running is not None and running.source_id == source_id:
                raise JobBusyError(running.id)
            return action()

    def _running(self) -> Job | None:
        return next((job for job in self._jobs.values() if job.state is JobState.RUNNING), None)

    def _trim(self) -> None:
        """Forget the oldest finished jobs beyond `keep` (never the running one)."""
        while len(self._jobs) > self._keep:
            oldest = next(
                (i for i, j in self._jobs.items() if j.state is not JobState.RUNNING), None
            )
            if oldest is None:
                return
            del self._jobs[oldest]

    def _run(self, job: Job) -> None:
        def progress(done: int, total: int) -> None:
            with self._lock:
                job.files_done, job.files_total = done, total

        try:
            report = self._indexer.sync(job.source_id, Path(job.root), on_progress=progress)
        except Exception as error:  # the job fails; the service must stay up
            logger.exception("index job %s failed", job.id)
            with self._lock:
                job.state = JobState.FAILED
                job.error = f"{type(error).__name__}: {error}"
        else:
            with self._lock:
                job.report = report
                job.state = JobState.DONE
