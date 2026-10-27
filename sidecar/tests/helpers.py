import time
from collections.abc import Callable

import numpy as np

from omni_memory.types import Chunk, ChunkKind, FileRecord

DIM = 8


def make_chunk(
    name: str,
    text: str = "",
    *,
    source_id: str = "s",
    kind: ChunkKind = ChunkKind.TEXT,
    mtime_ns: int = 1,
) -> Chunk:
    return Chunk(
        source_id=source_id,
        uri=f"{name}.txt",
        kind=kind,
        locator='{"lines":[1,1]}',
        content_hash=name,
        mtime_ns=mtime_ns,
        text=text or name,
    )


def unit_vectors(n: int, seed: int = 0, dim: int = DIM) -> np.ndarray:
    raw = np.random.default_rng(seed).standard_normal((n, dim)).astype(np.float32)
    return raw / np.linalg.norm(raw, axis=1, keepdims=True)


def make_file_chunks(uri: str, count: int, *, source_id: str = "s", tag: str = "v1") -> list[Chunk]:
    """`count` chunks of one file; changing `tag` changes every chunk's content (and id)."""
    return [
        Chunk(
            source_id=source_id,
            uri=uri,
            kind=ChunkKind.TEXT,
            locator=f'{{"i":{i}}}',
            content_hash=f"{uri}-{tag}-{i}",
            mtime_ns=1,
            text=f"{uri} {tag} part {i}",
        )
        for i in range(count)
    ]


def file_record(
    uri: str, *, source_id: str = "s", sha: str = "sha1", size: int = 1, mtime_ns: int = 1
) -> FileRecord:
    return FileRecord(source_id=source_id, uri=uri, size=size, mtime_ns=mtime_ns, sha256=sha)


def wait_for[T](predicate: Callable[[], T], timeout: float = 10.0, interval: float = 0.01) -> T:
    """Poll until `predicate()` is truthy and return that value; fail loudly on timeout."""
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if value := predicate():
            return value
        time.sleep(interval)
    raise AssertionError(f"condition not met within {timeout} s")
