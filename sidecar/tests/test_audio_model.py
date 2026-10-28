"""Audio through the real llama-server and models (`pytest -m model`, with OMNI_LLAMA_SERVER and
OMNI_MODEL_DIR set). Quality on speech is measured in docs/eval/2026-10-28-audio-embedding.md; these
check the plumbing that report relied on."""

import numpy as np
import pytest

from audio_fixture import make_wav, tone
from omni_memory.config import Settings
from omni_memory.embedding.llama import LlamaEmbedder
from omni_memory.embedding.llama_server import LlamaServer
from omni_memory.indexer import Indexer
from omni_memory.store import FlatStore
from omni_memory.types import ChunkKind, IndexOptions

pytestmark = pytest.mark.model
RATE = 16_000


@pytest.fixture(scope="module")
def embedder(tmp_path_factory):
    settings = Settings.from_env()
    assert settings.llama_server and settings.model_dir, "set OMNI_LLAMA_SERVER and OMNI_MODEL_DIR"
    server = LlamaServer(settings.llama_server, settings.model_dir)
    server.start()
    yield LlamaEmbedder(server, settings)
    server.close()


def noise(seconds: float, seed: int = 0) -> np.ndarray:
    return (np.random.default_rng(seed).standard_normal(int(seconds * RATE)) * 0.1).astype("f4")


def test_clips_are_unit_vectors_and_the_same_clip_gives_the_same_vector(embedder):
    clips = [tone(5), noise(5), tone(5)]
    vectors = embedder.embed_audio(clips)
    assert vectors.shape == (3, embedder.dim) and vectors.dtype == np.float32
    assert np.allclose(np.linalg.norm(vectors, axis=1), 1.0, atol=1e-5)
    assert float(vectors[0] @ vectors[2]) > 0.9999
    assert float(vectors[0] @ vectors[1]) < 0.99  # a tone and noise are different sounds


def test_a_full_thirty_second_window_is_accepted_by_the_server(embedder):
    # The server's batch holds about 40 s; 30 s is the limit this code allows itself.
    assert embedder.embed_audio([noise(30)]).shape == (1, embedder.dim)


def test_an_audio_file_is_indexed_through_the_real_model(embedder, tmp_path):
    root = tmp_path / "src"
    root.mkdir()
    make_wav(root / "hum.wav", 40)
    store = FlatStore(tmp_path / "idx", model_id=embedder.model_id, dim=embedder.dim)
    indexer = Indexer(store, embedder, Settings(index_dir=tmp_path / "idx"))
    report = indexer.sync("s", root, options=IndexOptions(audio="content"))
    assert report.errors == [] and report.chunks_written == 3  # the name and two windows
    chunks = store.get(store._catalog.chunk_ids_for_source("s"))
    kinds = {chunk.kind for chunk in chunks.values()}
    assert kinds == {ChunkKind.AUDIO, ChunkKind.AUDIO_WINDOW}
    store.close()
