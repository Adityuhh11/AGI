import subprocess
import sys
import textwrap

import pytest

from helpers import DIM, make_chunk, unit_vectors
from omni_memory.store import FlatStore


def test_failed_metadata_write_leaves_a_consistent_store(tmp_path, monkeypatch):
    store = FlatStore(tmp_path / "idx", model_id="fake", dim=DIM)
    kept = make_chunk("kept")
    store.upsert([kept], unit_vectors(1))

    def explode(self, chunks, rows):
        raise RuntimeError("disk on fire")

    with monkeypatch.context() as patch:
        patch.setattr(FlatStore, "_write_metadata", explode)
        lost = make_chunk("lost")
        with pytest.raises(RuntimeError, match="disk on fire"):
            store.upsert([lost], unit_vectors(1, seed=1))

    assert store.count() == 1
    assert store.get([lost.id]) == {}
    assert {h.id for h in store.search_vectors(unit_vectors(1, seed=1)[0], 10)} == {kept.id}
    store.upsert([lost], unit_vectors(1, seed=1))  # the orphaned row is reusable
    assert store.count() == 2
    store.close()


@pytest.mark.parametrize(
    "kill_point",
    ["FlatStore._write_metadata", "VectorFile.flush"],
    ids=["before-metadata-commit", "during-vector-flush"],
)
def test_hard_kill_mid_upsert(tmp_path, kill_point):
    """The process dies (no cleanup, no exception) at `kill_point` of the second upsert.

    Killing inside the vector flush proves vectors become durable before metadata commits.
    """
    directory = tmp_path / "idx"
    script = textwrap.dedent(
        f"""
        import os
        import sys
        from pathlib import Path

        import numpy as np

        from omni_memory.store import FlatStore
        from omni_memory.store.vectors import VectorFile
        from omni_memory.types import Chunk, ChunkKind

        def chunk(name):
            return Chunk("s", name + ".txt", ChunkKind.TEXT, "{{}}", name, 1, name)

        vec = np.zeros((1, {DIM}), dtype=np.float32)
        vec[0, 0] = 1.0
        store = FlatStore(Path(sys.argv[1]), model_id="fake", dim={DIM})
        store.upsert([chunk("committed")], vec)
        {kill_point} = lambda *args: os._exit(13)
        store.upsert([chunk("in-flight")], vec)
        """
    )
    result = subprocess.run(
        [sys.executable, "-I", "-c", script, str(directory)], capture_output=True, text=True
    )
    assert result.returncode == 13, result.stderr

    store = FlatStore(directory, model_id="fake", dim=DIM)
    assert store.count() == 1
    # exactly one result proves the in-flight vector row is ignored
    (survivor,) = store.search_vectors(unit_vectors(1)[0], 10)
    (kept,) = store.get([survivor.id]).values()
    assert kept.uri == "committed.txt"
    store.upsert([make_chunk("after")], unit_vectors(1, seed=4))  # and the store keeps working
    assert store.count() == 2
    store.close()
