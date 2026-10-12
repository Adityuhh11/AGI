"""Splits code identifiers into words so keyword search finds `addCountries` by 'countries'."""

from __future__ import annotations

import re

_IDENTIFIER = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")
# lower/digit -> Upper boundary, acronym -> Word boundary (HTTPServer), or underscores.
_BOUNDARY = re.compile(r"(?<=[a-z0-9])(?=[A-Z])|(?<=[A-Z])(?=[A-Z][a-z])|_+")


def split_identifiers(text: str) -> str:
    """Words inside multi-word identifiers, lowercase, unique, in order of first appearance."""
    words: dict[str, None] = {}
    for identifier in _IDENTIFIER.findall(text):
        parts = [part for part in _BOUNDARY.split(identifier) if part]
        if len(parts) < 2:
            continue
        for part in parts:
            words[part.lower()] = None
    return " ".join(words)
