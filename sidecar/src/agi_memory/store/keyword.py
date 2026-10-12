"""Turns free text into a safe FTS5 MATCH expression.

Every word is double-quoted, so user text can never be parsed as FTS5 syntax (NEAR, OR, ^, *,
column filters...). Words are OR-ed: bm25 ranks chunks matching more, rarer words first.
"""

from __future__ import annotations

import re

from agi_memory.store.identifiers import split_identifiers

_WORD = re.compile(r"\w+")
_STOPWORDS = frozenset(
    [
        "the",
        "a",
        "an",
        "of",
        "to",
        "in",
        "for",
        "and",
        "or",
        "is",
        "are",
        "how",
        "what",
        "do",
        "does",
        "with",
        "on",
        "at",
        "by",
        "from",
        "that",
        "this",
    ]
)
_MAX_WORDS = 32


def build_match(query: str) -> str:
    """FTS5 MATCH expression for `query`, or `""` if it has no searchable word."""
    words = _WORD.findall(query.lower()) + _WORD.findall(split_identifiers(query))
    kept = [w for w in dict.fromkeys(words) if len(w) > 1 and w not in _STOPWORDS]
    return " OR ".join(f'"{word}"' for word in kept[:_MAX_WORDS])
