import threading
import time

import numpy as np

from agi_memory.embedding import FakeEmbedder
from agi_memory.embedding.serialized import SerializedEmbedder


class Probe(FakeEmbedder):
    """Records how many calls are inside the embedder at the same moment."""

    def __init__(self) -> None:
        super().__init__(16)
        self.active = 0
        self.peak = 0
        self._guard = threading.Lock()

    def _inside(self) -> None:
        with self._guard:
            self.active += 1
            self.peak = max(self.peak, self.active)
        time.sleep(0.005)
        with self._guard:
            self.active -= 1

    def embed_documents(self, texts):
        self._inside()
        return super().embed_documents(texts)

    def embed_query(self, text):
        self._inside()
        return super().embed_query(text)


def hammer(embedder) -> None:
    def work():
        embedder.embed_documents(["a"])
        embedder.embed_query("b")

    threads = [threading.Thread(target=work) for _ in range(8)]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join()


def test_without_the_wrapper_calls_do_overlap():
    probe = Probe()
    hammer(probe)
    assert probe.peak > 1  # proves the next test can fail


def test_wrapped_calls_never_overlap():
    probe = Probe()
    hammer(SerializedEmbedder(probe))
    assert probe.peak == 1


def test_identity_and_results_pass_through():
    inner = FakeEmbedder(16)
    wrapped = SerializedEmbedder(inner)
    assert (wrapped.model_id, wrapped.dim, wrapped.device) == (inner.model_id, 16, "cpu")
    np.testing.assert_array_equal(wrapped.embed_query("x"), inner.embed_query("x"))
    np.testing.assert_array_equal(
        wrapped.embed_documents(["x", "y"]), inner.embed_documents(["x", "y"])
    )
    assert wrapped.embed_images([]).shape == (0, 16)
