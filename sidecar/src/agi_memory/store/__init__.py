"""Persistent index: chunk metadata (SQLite) and vectors (memory-mapped file)."""

from agi_memory.store.flat import FlatStore, IndexMismatchError

__all__ = ["FlatStore", "IndexMismatchError"]
