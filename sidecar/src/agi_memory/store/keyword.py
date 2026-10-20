"""Keyword search (bm25 over text, identifiers and path words) and path search (trigram substring
over paths), as SQL over the store's open connection. `FlatStore` holds the lock around them.

User text becomes a safe FTS5 MATCH expression: every word is double-quoted, so it can never be
parsed as FTS5 syntax (NEAR, OR, ^, *, column filters...). Words are OR-ed: bm25 ranks chunks
matching more, rarer words first. Phrases are added as extra terms, so text that contains the words
side by side outranks text that merely contains them: every punctuated token (`refresh-token`,
`react-leaflet`) and, for short queries, the whole query (an error message or sentence copied from
a file).
"""

from __future__ import annotations

import re
import sqlite3
from collections.abc import Collection, Sequence

from agi_memory.store.filters import filter_clause
from agi_memory.store.identifiers import split_identifiers
from agi_memory.types import Filters, KeywordHit

# A path needle found in more chunks or files than this (`txt`, `.js`, `src`) is a common
# fragment, not a file name: it carries no signal and scoring every match is slow at scale.
_MAX_PATH_CHUNKS = 1000
_MAX_PATH_FILES = 50
# A query word in more than this share of the chunks (and in over `_COMMON_MIN_DOCS` of them) has
# almost no weight in bm25 but still costs a full scan; keyword search leaves it out.
_COMMON_FRACTION = 0.2
_COMMON_MIN_DOCS = 1000

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


def keyword_search(
    db: sqlite3.Connection, query: str, n: int, filters: Filters | None
) -> list[KeywordHit]:
    """Top-`n` chunks by bm25 over text, identifiers and path words, ordered by (-score, id)."""
    if n <= 0:
        return []
    match = build_match(query, _common_words(db, search_words(query)))
    if not match:
        return []
    sql = (
        "SELECT c.id, bm25(chunks_fts, 1.0, 0.5, 2.0) AS score FROM chunks_fts "
        "JOIN chunks c ON c.rid = chunks_fts.rowid WHERE chunks_fts MATCH ?"
    )
    params: list[object] = [match]
    clause = filter_clause(filters) if filters is not None else None
    if clause is not None:
        sql += f" AND ({clause[0]})"
        params.extend(clause[1])
    sql += " ORDER BY score, c.id LIMIT ?"
    params.append(n)
    return [KeywordHit(id=chunk_id, score=-score) for chunk_id, score in db.execute(sql, params)]


def _common_words(db: sqlite3.Connection, words: Sequence[str]) -> set[str]:
    """The `words` too common to rank by (one vocabulary lookup)."""
    if not words:
        return set()
    (newest,) = db.execute("SELECT COALESCE(MAX(rid), 0) FROM chunks").fetchone()
    limit = max(_COMMON_MIN_DOCS, newest * _COMMON_FRACTION)  # newest >= live count: safe side
    marks = ",".join("?" * len(words))
    rows = db.execute(
        f"SELECT term FROM chunks_vocab WHERE term IN ({marks}) AND doc > ?", [*words, limit]
    )
    return {term for (term,) in rows}


def path_search(db: sqlite3.Connection, query: str, filters: Filters | None) -> list[KeywordHit]:
    """Chunks whose path contains the whole query as a substring (case-insensitive).

    This is what finds a file by typing its name, or part of it. Every chunk of a matching file
    matches, with score 1.0 so that the embedding decides the order among them. Nothing matches for
    queries under 3 characters, or when the fragment is found in more than `_MAX_PATH_CHUNKS` chunks
    or `_MAX_PATH_FILES` files of the searched scope.
    """
    needle = query.strip().replace('"', '""')
    if len(needle) < 3:
        return []
    sql = (
        "SELECT chunks.id, chunks.uri FROM "
        "(SELECT rowid AS rid FROM chunks_path_tri WHERE chunks_path_tri MATCH ?) AS m "
        "JOIN chunks ON chunks.rid = m.rid"
    )
    params: list[object] = [f'"{needle}"']
    clause = filter_clause(filters) if filters is not None else None
    if clause is not None:
        sql += f" WHERE ({clause[0]})"
        params.extend(clause[1])
    sql += " LIMIT ?"  # stops early on a fragment found everywhere
    params.append(_MAX_PATH_CHUNKS + 1)
    rows = db.execute(sql, params).fetchall()
    if len(rows) > _MAX_PATH_CHUNKS or len({uri for _, uri in rows}) > _MAX_PATH_FILES:
        return []
    return [KeywordHit(id=chunk_id, score=1.0) for chunk_id in sorted(i for i, _ in rows)]
