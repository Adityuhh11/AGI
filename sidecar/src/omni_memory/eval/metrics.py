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
    confident: bool = True  # the search's own "this looks like an answer" flag
    answerable: bool = True  # False for a golden query that nothing in the corpus answers


@dataclass(frozen=True, slots=True)
class Metrics:
    n: int
    top1: float
    recall3: float
    recall10: float
    mrr: float
    forbidden_hits: int
    false_alarms: float = 0.0  # share of answerable queries the search flagged as not confident
    missed_nones: float = 0.0  # share of unanswerable queries the search flagged as confident


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
    answerable = [r for r in results if r.answerable]
    unanswerable = [r for r in results if not r.answerable]
    ranks = [r.first_rank for r in answerable]
    count = len(answerable) or 1  # ranking metrics are over answerable queries only
    return Metrics(
        n=n,
        top1=sum(1 for r in ranks if r == 1) / count,
        recall3=sum(1 for r in ranks if r is not None and r <= 3) / count,
        recall10=sum(1 for r in ranks if r is not None and r <= 10) / count,
        mrr=sum(1 / r for r in ranks if r is not None) / count,
        forbidden_hits=sum(len(r.forbidden) for r in results),
        false_alarms=sum(1 for r in answerable if not r.confident) / count,
        missed_nones=sum(1 for r in unanswerable if r.confident) / (len(unanswerable) or 1),
    )
