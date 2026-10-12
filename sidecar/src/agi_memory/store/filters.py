"""SQL for `Filters`, shared by vector search (row masks) and keyword search (joins)."""

from __future__ import annotations

from agi_memory.types import Filters


def filter_clause(filters: Filters) -> tuple[str, list[object]] | None:
    """A WHERE fragment over the `chunks` columns and its parameters; None if unrestricted."""
    clauses: list[str] = []
    params: list[object] = []
    if filters.kinds is not None:
        if not filters.kinds:
            return "0", []  # an empty set of allowed kinds matches nothing
        clauses.append(f"kind IN ({','.join('?' * len(filters.kinds))})")
        params.extend(sorted(kind.value for kind in filters.kinds))
    if filters.source_id is not None:
        clauses.append("source_id = ?")
        params.append(filters.source_id)
    if filters.uri_prefix is not None:
        clauses.append("substr(uri, 1, ?) = ?")  # substr, not LIKE: '%' and '_' are literal
        params.extend([len(filters.uri_prefix), filters.uri_prefix])
    if filters.modified_after_ns is not None:
        clauses.append("mtime_ns > ?")
        params.append(filters.modified_after_ns)
    if not clauses:
        return None
    return " AND ".join(clauses), params
