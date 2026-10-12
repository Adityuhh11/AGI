"""Chunks for text-like files: source code, plain text, CSV/JSON and Markdown.

Baseline strategy (measured later against the golden set): fixed windows of lines with overlap,
Markdown split by heading first. Every chunk's text starts with the file's relative path so the
path is both embedded and keyword-searchable.
"""

from __future__ import annotations

import re
from collections.abc import Iterator
from itertools import pairwise

from agi_memory.chunking.common import ChunkingError, locator_json, sha256_hex
from agi_memory.config import Settings
from agi_memory.sources.rules import Route
from agi_memory.types import Chunk, ChunkKind

_KINDS = {
    Route.CODE: ChunkKind.CODE,
    Route.MARKDOWN: ChunkKind.MARKDOWN,
    Route.TEXT: ChunkKind.TEXT,
}
_HEADING = re.compile(r"^#{1,6}\s")
_BINARY_PROBE = 8192


class NotTextError(ChunkingError):
    """The file looks binary (NUL bytes), so it is skipped rather than indexed as garbage."""


def chunk_text_document(
    source_id: str, uri: str, route: Route, data: bytes, mtime_ns: int, settings: Settings
) -> list[Chunk]:
    """Chunks for one text-like file (`route` must be CODE, MARKDOWN or TEXT)."""
    kind = _KINDS.get(route)
    if kind is None:
        raise ValueError(f"route {route!r} is not a text route")
    if b"\0" in data[:_BINARY_PROBE]:
        raise NotTextError(f"{uri} looks binary")
    lines = data.decode("utf-8", errors="replace").splitlines()
    if route is Route.MARKDOWN:
        spans = _markdown_spans(lines, settings)
    else:
        spans = list(_windows(0, len(lines), settings))
    chunks = []
    for first, last in spans:
        body = "\n".join(lines[first - 1 : last])
        if not body.strip():
            continue
        text = f"{uri}\n{body}"[: settings.max_chunk_chars]
        chunks.append(
            Chunk(
                source_id=source_id,
                uri=uri,
                kind=kind,
                locator=locator_json(lines=[first, last]),
                content_hash=sha256_hex(text),
                mtime_ns=mtime_ns,
                text=text,
            )
        )
    return chunks


def _windows(start: int, stop: int, settings: Settings) -> Iterator[tuple[int, int]]:
    """Overlapping windows over 0-based [start, stop), yielded as 1-based inclusive ranges."""
    step = settings.chunk_lines - settings.chunk_overlap_lines
    position = start
    while position < stop:
        end = min(position + settings.chunk_lines, stop)
        yield position + 1, end
        if end == stop:
            return
        position += step


def _markdown_spans(lines: list[str], settings: Settings) -> list[tuple[int, int]]:
    """One span per heading section; sections longer than a window are windowed."""
    bounds = [0]
    in_fence = False
    for index, line in enumerate(lines):
        if line.lstrip().startswith("```"):
            in_fence = not in_fence
        elif not in_fence and index > 0 and _HEADING.match(line):
            bounds.append(index)
    bounds.append(len(lines))
    spans: list[tuple[int, int]] = []
    for start, stop in pairwise(bounds):
        spans.extend(_windows(start, stop, settings))
    return spans
