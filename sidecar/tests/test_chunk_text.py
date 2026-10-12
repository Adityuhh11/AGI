import hashlib
import json

import pytest

from agi_memory.chunking.common import locator_json
from agi_memory.chunking.text import NotTextError, chunk_text_document
from agi_memory.config import Settings
from agi_memory.sources.rules import Route
from agi_memory.types import ChunkKind


@pytest.fixture
def settings(tmp_path):
    return Settings(index_dir=tmp_path)


def chunk(settings, route, text, uri="src/a.js"):
    return chunk_text_document("repo", uri, route, text.encode(), 7, settings)


def numbered(count):
    return "\n".join(f"line {i}" for i in range(1, count + 1))


def line_ranges(chunks):
    return [json.loads(c.locator)["lines"] for c in chunks]


def test_code_windows_overlap_by_ten_lines(settings):
    chunks = chunk(settings, Route.CODE, numbered(100))
    assert line_ranges(chunks) == [[1, 40], [31, 70], [61, 100]]
    assert all(c.kind is ChunkKind.CODE for c in chunks)


def test_short_file_is_one_chunk_and_exact_window_is_not_split(settings):
    assert line_ranges(chunk(settings, Route.CODE, numbered(40))) == [[1, 40]]
    assert line_ranges(chunk(settings, Route.CODE, numbered(41))) == [[1, 40], [31, 41]]


def test_chunk_text_starts_with_the_uri_and_metadata_is_carried(settings):
    first = chunk(settings, Route.CODE, numbered(3))[0]
    assert first.text == "src/a.js\nline 1\nline 2\nline 3"
    assert first.locator == '{"lines":[1,3]}'
    assert (first.source_id, first.uri, first.mtime_ns) == ("repo", "src/a.js", 7)
    assert first.content_hash == hashlib.sha256(first.text.encode()).hexdigest()


def test_ids_are_deterministic_and_differ_per_window(settings):
    a = chunk(settings, Route.CODE, numbered(100))
    b = chunk(settings, Route.CODE, numbered(100))
    assert [c.id for c in a] == [c.id for c in b]
    assert len({c.id for c in a}) == 3


@pytest.mark.parametrize("content", ["", "   \n\n\t\n"])
def test_empty_and_blank_files_produce_no_chunks(settings, content):
    assert chunk(settings, Route.TEXT, content) == []


def test_crlf_line_endings_are_normalised(settings):
    (only,) = chunk(settings, Route.TEXT, "a\r\nb\r\n", uri="x.txt")
    assert only.text == "x.txt\na\nb"


def test_binary_data_is_not_text(settings):
    with pytest.raises(NotTextError):
        chunk_text_document("repo", "a.js", Route.CODE, b"\x00\x01\x02binary", 1, settings)


def test_invalid_utf8_is_replaced_not_fatal(settings):
    (only,) = chunk_text_document("repo", "a.txt", Route.TEXT, b"ok \xff\xfe bad", 1, settings)
    assert "\ufffd" in only.text


def test_one_enormous_line_is_capped(settings):
    (only,) = chunk(settings, Route.CODE, "x" * 1_000_000)
    assert len(only.text) == settings.max_chunk_chars


def test_text_route_gives_text_kind_and_other_routes_are_rejected(settings):
    assert chunk(settings, Route.TEXT, "a,b\n1,2", uri="d.csv")[0].kind is ChunkKind.TEXT
    with pytest.raises(ValueError, match="route"):
        chunk(settings, Route.PDF, "x")


def test_markdown_splits_by_heading(settings):
    doc = "# Title\nintro\n## A\nalpha\n## B\nbeta"
    chunks = chunk(settings, Route.MARKDOWN, doc, uri="r.md")
    assert line_ranges(chunks) == [[1, 2], [3, 4], [5, 6]]
    assert all(c.kind is ChunkKind.MARKDOWN for c in chunks)


def test_markdown_ignores_hashes_inside_code_fences(settings):
    doc = "# T\n```\n# not a heading\n```\ntext"
    assert line_ranges(chunk(settings, Route.MARKDOWN, doc, uri="r.md")) == [[1, 5]]


def test_markdown_text_before_the_first_heading_is_kept(settings):
    doc = "preamble\n# H\nbody"
    assert line_ranges(chunk(settings, Route.MARKDOWN, doc, uri="r.md")) == [[1, 1], [2, 3]]


def test_a_long_markdown_section_is_windowed(settings):
    doc = "# H\n" + numbered(100)  # 101 lines in one section
    ranges = line_ranges(chunk(settings, Route.MARKDOWN, doc, uri="r.md"))
    assert ranges == [[1, 40], [31, 70], [61, 100], [91, 101]]


def test_locator_json_is_canonical():
    assert locator_json(page=3, extra=None) == '{"extra":null,"page":3}'
