"""Files the engine cannot read (Office files, archives): one chunk whose text is the path."""

from __future__ import annotations

import hashlib

from omni_memory.types import Chunk, ChunkKind


def name_identity(size: int) -> str:
    """Stands in for a content hash: the name is all that is indexed, so the size is enough to
    notice a moved file without opening it."""
    return hashlib.sha256(str(size).encode()).hexdigest()


def file_name_chunk(source_id: str, uri: str, identity: str, mtime_ns: int) -> Chunk:
    return Chunk(
        source_id=source_id,
        uri=uri,
        kind=ChunkKind.FILE,
        locator="{}",
        content_hash=f"name:{identity}",
        mtime_ns=mtime_ns,
        text=uri,
    )
