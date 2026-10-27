"""Small helpers shared by every chunker."""

from __future__ import annotations

import hashlib
import json


class ChunkingError(Exception):
    """A file could not be turned into chunks (corrupt, encrypted, undecodable...)."""


def locator_json(**fields: object) -> str:
    """Canonical JSON for a chunk locator so equal inputs give equal ids."""
    return json.dumps(fields, separators=(",", ":"), sort_keys=True)


def sha256_hex(text: str) -> str:
    return hashlib.sha256(text.encode()).hexdigest()
