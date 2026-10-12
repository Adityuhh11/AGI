import numpy as np

from agi_memory.types import Chunk, ChunkKind

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
