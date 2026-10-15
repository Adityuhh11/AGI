"""Turns free text into a safe FTS5 MATCH expression.

Every word is double-quoted, so user text can never be parsed as FTS5 syntax (NEAR, OR, ^, *,
column filters...). Words are OR-ed: bm25 ranks chunks matching more, rarer words first. Phrases
are added as extra terms, so text that contains the words side by side outranks text that merely
contains them: every punctuated token (`refresh-token`, `react-leaflet`) and, for short queries,
the whole query (an error message or sentence copied from a file).
"""

from __future__ import annotations

import re

from agi_memory.store.identifiers import split_identifiers

_WORD = re.compile(r"\w+")
_NOT_WORD = re.compile(r"\W")
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
_MAX_PHRASE_QUERY_WORDS = 8


def build_match(query: str) -> str:
    """FTS5 MATCH expression for `query`, or `""` if it has no searchable word."""
    words = _WORD.findall(query.lower()) + _WORD.findall(split_identifiers(query))
    kept = [w for w in dict.fromkeys(words) if len(w) > 1 and w not in _STOPWORDS]
    if not kept:
        return ""
    terms = [f'"{word}"' for word in kept[:_MAX_WORDS]]
    terms.extend(f'"{" ".join(phrase)}"' for phrase in _phrases(query))
    return " OR ".join(dict.fromkeys(terms))


def _phrases(query: str) -> list[list[str]]:
    found = []
    for token in query.split():
        parts = _WORD.findall(token.lower())
        if len(parts) > 1 and _NOT_WORD.search(token):
            found.append(parts)
    whole = _WORD.findall(query.lower())
    if 2 <= len(whole) <= _MAX_PHRASE_QUERY_WORDS:
        found.append(whole)
    return found
