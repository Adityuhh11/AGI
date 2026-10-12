import dataclasses

import pytest

from agi_memory.types import Chunk, ChunkKind, chunk_id


def make(**overrides) -> Chunk:
    fields = dict(
        source_id="repo",
        uri="src/a.js",
        kind=ChunkKind.CODE,
        locator='{"lines":[1,40]}',
        content_hash="abc",
        mtime_ns=1,
        text="hello",
    )
    return Chunk(**{**fields, **overrides})


def test_id_is_deterministic():
    assert make().id == make().id
    assert make().id == chunk_id("repo", "src/a.js", '{"lines":[1,40]}', "abc")


def test_id_ignores_fields_that_are_not_identity():
    assert make(mtime_ns=999, text="different").id == make().id


@pytest.mark.parametrize(
    "override",
    [
        {"source_id": "other"},
        {"uri": "src/b.js"},
        {"locator": '{"lines":[31,70]}'},
        {"content_hash": "changed"},
    ],
)
def test_id_changes_with_identity_fields(override):
    assert make(**override).id != make().id


def test_chunk_is_immutable():
    with pytest.raises(dataclasses.FrozenInstanceError):
        make().text = "nope"  # type: ignore[misc]
