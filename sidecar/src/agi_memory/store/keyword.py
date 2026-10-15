"""Turns free text into a safe FTS5 MATCH expression.

Every word is double-quoted, so user text can never be parsed as FTS5 syntax (NEAR, OR, ^, *,
column filters...). Words are OR-ed: bm25 ranks chunks matching more, rarer words first. Phrases
are added as extra terms, so text that contains the words side by side outranks text that merely
contains them: every punctuated token (`refresh-token`, `react-leaflet`) and, for short queries,
the whole query (an error message or sentence copied from a file).
"""

from __future__ import annotations

import re
from collections.abc import Collection

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
_MAX_PUNCTUATED_PHRASES = 8


def search_words(query: str) -> list[str]:
    """The distinct words of `query` worth matching, identifier parts included, capped."""
    words = _WORD.findall(query.lower()) + _WORD.findall(split_identifiers(query))
    kept = [w for w in dict.fromkeys(words) if len(w) > 1 and w not in _STOPWORDS]
    return kept[:_MAX_WORDS]


def build_match(query: str, skip: Collection[str] = ()) -> str:
    """FTS5 MATCH expression for `query`, or `""` if it has no searchable word.

    `skip` holds words that occur in so many chunks that bm25 would score most of the index for
    almost no signal; the caller knows the corpus, this function does not.
    """
    kept = [word for word in search_words(query) if word not in skip]
    if not kept:
        return ""
    terms = [f'"{word}"' for word in kept]
    terms.extend(f'"{" ".join(phrase)}"' for phrase in _phrases(query))
    return " OR ".join(dict.fromkeys(terms))


def _phrases(query: str) -> list[list[str]]:
    found = []
    for token in query.split():
        parts = _WORD.findall(token.lower())
        if len(parts) > 1 and _NOT_WORD.search(token):
            found.append(parts)
    found = found[:_MAX_PUNCTUATED_PHRASES]  # pasted code has hundreds of `a.b` tokens
    whole = _WORD.findall(query.lower())
    if 2 <= len(whole) <= _MAX_PHRASE_QUERY_WORDS:
        found.append(whole)
    return found
