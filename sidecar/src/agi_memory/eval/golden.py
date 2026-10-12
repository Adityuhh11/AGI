"""The golden query set: queries with known-good answers, kept in a reviewable YAML file."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

import yaml

_KEYS = {"query", "expect", "forbid", "note"}


@dataclass(frozen=True, slots=True)
class GoldenQuery:
    query: str
    expect: tuple[str, ...]  # a hit is relevant if its uri contains any of these
    forbid: tuple[str, ...]  # no top-k hit's uri may contain any of these (privacy checks)
    note: str


def _strings(value: object, where: str) -> tuple[str, ...]:
    if value is None:
        return ()
    if not isinstance(value, list) or not all(isinstance(item, str) and item for item in value):
        raise ValueError(f"golden {where} must be a list of non-empty strings")
    return tuple(value)


def load_golden(path: Path) -> list[GoldenQuery]:
    """Parse and validate a golden file; a malformed file is an error, never a silent skip."""
    raw = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(raw, list):
        raise ValueError(f"golden file {path} must be a list of queries")
    queries: list[GoldenQuery] = []
    seen: set[str] = set()
    for number, entry in enumerate(raw, start=1):
        where = f"entry {number}"
        if not isinstance(entry, dict):
            raise ValueError(f"golden {where} must be a mapping")
        unknown = set(entry) - _KEYS
        if unknown:
            raise ValueError(f"golden {where} has unknown keys {sorted(unknown)}")
        query = entry.get("query")
        if not isinstance(query, str) or not query.strip():
            raise ValueError(f"golden {where} needs a non-empty query")
        expect = _strings(entry.get("expect"), f"{where} expect")
        if not expect:
            raise ValueError(f"golden {where} needs at least one expect string")
        if query in seen:
            raise ValueError(f"golden query repeated: {query!r}")
        seen.add(query)
        note = entry.get("note", "")
        queries.append(
            GoldenQuery(query, expect, _strings(entry.get("forbid"), f"{where} forbid"), str(note))
        )
    return queries
