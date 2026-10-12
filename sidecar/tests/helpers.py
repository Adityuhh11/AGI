import numpy as np

from agi_memory.types import Chunk, ChunkKind, FileRecord

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
