"""Small helpers shared by every chunker."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path

_EDGE = 1 << 20  # bytes hashed from each end of a media file


class ChunkingError(Exception):
    """A file could not be turned into chunks (corrupt, encrypted, undecodable...)."""


def locator_json(**fields: object) -> str:
    """Canonical JSON for a chunk locator so equal inputs give equal ids."""
    return json.dumps(fields, separators=(",", ":"), sort_keys=True)


def sha256_hex(text: str) -> str:
    return hashlib.sha256(text.encode()).hexdigest()


def media_identity(path: Path, size: int) -> str:
    """A content hash of the size and the first and last mebibyte: enough to notice that a video
    or an audio file moved without reading gigabytes."""
    digest = hashlib.sha256(str(size).encode())
    with path.open("rb") as file:
        digest.update(file.read(_EDGE))
        if size > 2 * _EDGE:
            file.seek(size - _EDGE)
        digest.update(file.read(_EDGE))
    return digest.hexdigest()
