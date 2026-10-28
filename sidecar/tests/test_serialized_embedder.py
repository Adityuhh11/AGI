import threading
import time

import numpy as np
import pytest
from PIL import Image

from omni_memory.embedding import FakeEmbedder
from omni_memory.embedding.serialized import SerializedEmbedder


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

    def embed_documents(self, texts, titles=None):
        self._inside()
        return super().embed_documents(texts, titles)

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


class Metered(FakeEmbedder):
    """Takes time in proportion to the work, and remembers how big each call was."""

    def __init__(self) -> None:
        super().__init__(16)
        self.document_calls: list[int] = []
        self.image_calls: list[int] = []

    def embed_documents(self, texts, titles=None):
        self.document_calls.append(len(texts))
        time.sleep(0.01 * len(texts))
        return super().embed_documents(texts, titles)

    def embed_images(self, images):
        self.image_calls.append(len(images))
        time.sleep(0.01 * len(images))
        return super().embed_images(images)


def test_sliced_bulk_calls_give_the_same_vectors_in_the_same_order():
    texts = [f"document number {i}" for i in range(21)]
    titles = [f"file{i}.txt" for i in range(21)]
    plain = FakeEmbedder(16)
    metered = Metered()
    wrapped = SerializedEmbedder(metered, text_slice=4)
    np.testing.assert_array_equal(
        wrapped.embed_documents(texts, titles), plain.embed_documents(texts, titles)
    )
    np.testing.assert_array_equal(wrapped.embed_documents(texts), plain.embed_documents(texts))
    assert max(metered.document_calls) == 4
    assert sum(metered.document_calls) == 42


def test_images_go_through_in_slices_and_empty_inputs_keep_their_shape():
    images = [Image.new("RGB", (8, 8), (i * 20, 0, 0)) for i in range(5)]
    metered = Metered()
    wrapped = SerializedEmbedder(metered, image_slice=2)
    np.testing.assert_array_equal(
        wrapped.embed_images(images), FakeEmbedder(16).embed_images(images)
    )
    assert metered.image_calls == [2, 2, 1]
    assert wrapped.embed_images([]).shape == (0, 16)
    assert wrapped.embed_documents([]).shape == (0, 16)


def test_a_mismatched_title_count_is_still_an_error():
    with pytest.raises(ValueError, match="titles"):
        SerializedEmbedder(FakeEmbedder(16)).embed_documents(["a", "b"], ["only one"])


def test_a_query_waits_for_one_slice_not_for_the_whole_bulk_call():
    wrapped = SerializedEmbedder(Metered(), text_slice=4)
    bulk = threading.Thread(target=wrapped.embed_documents, args=([f"t{i}" for i in range(120)],))
    bulk.start()  # about 1.2 s of work in 4-text slices of 40 ms
    time.sleep(0.15)
    started = time.perf_counter()
    wrapped.embed_query("hello")
    waited = time.perf_counter() - started
    bulk.join()
    assert (
        waited < 0.3
    )  # one slice plus scheduling; a single 1.2 s call would block for about a second


def test_a_stream_of_queries_cannot_starve_or_corrupt_a_bulk_call():
    texts = [f"t{i}" for i in range(40)]
    wrapped = SerializedEmbedder(Metered(), text_slice=4)
    result: list[np.ndarray] = []
    bulk = threading.Thread(target=lambda: result.append(wrapped.embed_documents(texts)))
    bulk.start()
    for _ in range(10):
        wrapped.embed_query("question")
    bulk.join(timeout=10)
    assert not bulk.is_alive()
    np.testing.assert_array_equal(result[0], FakeEmbedder(16).embed_documents(texts))


def test_audio_goes_through_in_slices():
    clips = [np.full(160, i / 10, "f4") for i in range(5)]
    calls = []

    class Counting(FakeEmbedder):
        def embed_audio(self, clips):
            calls.append(len(clips))
            return super().embed_audio(clips)

    wrapped = SerializedEmbedder(Counting(16), image_slice=2)
    np.testing.assert_array_equal(wrapped.embed_audio(clips), FakeEmbedder(16).embed_audio(clips))
    assert calls == [2, 2, 1] and wrapped.embed_audio([]).shape == (0, 16)
