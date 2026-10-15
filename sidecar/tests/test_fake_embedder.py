import numpy as np
import pytest
from PIL import Image

from agi_memory.embedding import FakeEmbedder


def test_rows_are_float32_unit_vectors():
    vecs = FakeEmbedder().embed_documents(["alpha beta", "gamma"])
    assert vecs.shape == (2, 256)
    assert vecs.dtype == np.float32
    np.testing.assert_allclose(np.linalg.norm(vecs, axis=1), 1.0, atol=1e-5)


def test_same_text_same_vector_across_instances():
    a = FakeEmbedder().embed_documents(["the quick fox"])
    b = FakeEmbedder().embed_documents(["the quick fox"])
    np.testing.assert_array_equal(a, b)


def test_shared_words_mean_higher_similarity():
    fake = FakeEmbedder()
    query = fake.embed_query("goroutine channels")
    related, unrelated = fake.embed_documents(
        ["channels connect goroutine workers", "recipe for chocolate cake"]
    )
    assert float(query @ related) > float(query @ unrelated) + 0.3


def test_query_has_shape_dim():
    assert FakeEmbedder(dim=64).embed_query("x").shape == (64,)


def test_empty_input_list_gives_empty_matrix():
    assert FakeEmbedder().embed_documents([]).shape == (0, 256)
    assert FakeEmbedder().embed_images([]).shape == (0, 256)


def test_empty_string_is_still_a_unit_vector():
    vec = FakeEmbedder().embed_query("")
    assert np.isfinite(vec).all()
    assert abs(float(np.linalg.norm(vec)) - 1.0) < 1e-5


def test_images_are_embedded_by_content():
    fake = FakeEmbedder()
    red = Image.new("RGB", (8, 8), "red")
    blue = Image.new("RGB", (8, 8), "blue")
    a, b, c = fake.embed_images([red, Image.new("RGB", (8, 8), "red"), blue])
    np.testing.assert_array_equal(a, b)
    assert float(a @ c) < 0.5


def test_fake_embedder_reports_a_cpu_device():
    assert FakeEmbedder(8).device == "cpu"


def test_titles_change_the_vector_and_no_titles_changes_nothing():
    fake = FakeEmbedder(32)
    plain = fake.embed_documents(["alpha"])
    np.testing.assert_array_equal(fake.embed_documents(["alpha"], None), plain)
    titled = fake.embed_documents(["alpha"], ["report"])
    assert not np.array_equal(titled, plain)
    query = fake.embed_query("report")
    assert (titled @ query)[0] > (plain @ query)[0]  # the title words are part of the document


def test_a_title_count_that_does_not_match_is_an_error():
    with pytest.raises(ValueError, match="titles"):
        FakeEmbedder(8).embed_documents(["a", "b"], ["only one"])
