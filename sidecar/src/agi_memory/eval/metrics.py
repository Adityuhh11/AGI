"""Retrieval metrics over golden-query results."""

from __future__ import annotations

from collections.abc import Sequence
from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class QueryResult:
    query: str
    first_rank: int | None  # 1-based rank of the first relevant hit within the top-k
    forbidden: tuple[str, ...]  # forbidden strings found in top-k uris
    top_uris: tuple[str, ...]


@dataclass(frozen=True, slots=True)
class Metrics:
    n: int
    top1: float
    recall3: float
    recall10: float
    mrr: float
    forbidden_hits: int


def first_relevant_rank(uris: Sequence[str], expect: Sequence[str]) -> int | None:
    """1-based position of the first uri containing any `expect` substring, else None."""
    for rank, uri in enumerate(uris, start=1):
        if any(fragment in uri for fragment in expect):
            return rank
    return None


def summarize(results: Sequence[QueryResult]) -> Metrics:
    n = len(results)
    if n == 0:
        return Metrics(0, 0.0, 0.0, 0.0, 0.0, 0)
    ranks = [r.first_rank for r in results]
    return Metrics(
        n=n,
        top1=sum(1 for r in ranks if r == 1) / n,
        recall3=sum(1 for r in ranks if r is not None and r <= 3) / n,
        recall10=sum(1 for r in ranks if r is not None and r <= 10) / n,
        mrr=sum(1 / r for r in ranks if r is not None) / n,
        forbidden_hits=sum(len(r.forbidden) for r in results),
    )
