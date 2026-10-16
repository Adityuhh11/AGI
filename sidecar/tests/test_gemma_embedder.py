import numpy as np
import pytest
import torch
from PIL import Image, ImageDraw

from agi_memory.config import EMBEDDING_DIM, Settings
from agi_memory.embedding.gemma import GemmaEmbedder

pytestmark = pytest.mark.model

DOCS = [
    "Go channels let goroutines communicate safely.",
    "My flight to Hyderabad leaves at 6am on Friday.",
    "Svelte compiles components into small vanilla JS.",
    "Remember to buy milk and eggs.",
]


@pytest.fixture(scope="module")
def settings(tmp_path_factory):
    return Settings(index_dir=tmp_path_factory.mktemp("idx"))


@pytest.fixture(scope="module")
def embedder(settings):
    return GemmaEmbedder(settings)


def test_vectors_are_float32_unit_rows(embedder):
    vecs = embedder.embed_documents(DOCS)
    assert vecs.shape == (4, EMBEDDING_DIM)
    assert vecs.dtype == np.float32
    np.testing.assert_allclose(np.linalg.norm(vecs, axis=1), 1.0, atol=1e-5)


def test_related_text_ranks_first(embedder):
    query = embedder.embed_query("how do goroutines talk to each other")
    scores = embedder.embed_documents(DOCS) @ query
    assert int(np.argmax(scores)) == 0
    assert scores[0] > 0.75  # spike measured 0.84
    assert np.sort(scores)[-2] < 0.7  # spike measured <= 0.60 for the others


def test_a_title_is_part_of_what_a_document_means(embedder):
    texts = ["Quarterly numbers and notes."] * 2
    docs = embedder.embed_documents(texts, ["Budget 2026.xlsx", "Holiday photos.txt"])
    query = embedder.embed_query("annual budget spreadsheet")
    assert docs[0] @ query > docs[1] @ query
    with pytest.raises(ValueError, match="titles"):
        embedder.embed_documents(["a", "b"], ["x"])


def test_text_finds_matching_image(embedder):
    image = Image.new("RGB", (512, 512), "white")
    ImageDraw.Draw(image).rectangle([100, 300, 400, 320], fill="red")
    (image_vec,) = embedder.embed_images([image])
    red = embedder.embed_query("a red line on a white background") @ image_vec
    cat = embedder.embed_query("a photo of a cat") @ image_vec
    assert red > cat + 0.1  # spike measured 0.775 vs 0.559


def test_same_input_gives_same_vector(embedder):
    a = embedder.embed_documents(DOCS[:2])
    b = embedder.embed_documents(DOCS[:2])
    np.testing.assert_allclose(a, b, atol=1e-3)


def test_empty_inputs(embedder):
    assert embedder.embed_documents([]).shape == (0, EMBEDDING_DIM)
    assert embedder.embed_images([]).shape == (0, EMBEDDING_DIM)


@pytest.mark.skipif(not torch.cuda.is_available(), reason="needs CUDA")
def test_peak_vram_within_budget(embedder):
    torch.cuda.reset_peak_memory_stats()
    long_texts = ["word " * 600] * 16  # truncated to 512 tokens
    embedder.embed_documents(long_texts)
    embedder.embed_images([Image.new("RGB", (512, 512), "gray")] * 4)
    assert torch.cuda.max_memory_allocated() / 2**30 <= 1.5


@pytest.mark.skipif(not torch.cuda.is_available(), reason="needs CUDA")
def test_cpu_and_gpu_vectors_agree(embedder, settings):
    cpu = GemmaEmbedder(settings, device="cpu")
    assert embedder.device == "cuda" and cpu.device == "cpu"
    cosine = np.sum(embedder.embed_documents(DOCS) * cpu.embed_documents(DOCS), axis=1)
    print(f"min GPU(bf16) vs CPU(fp32) cosine: {cosine.min():.4f}")  # recorded in docs/STATUS.md
    assert cosine.min() > 0.98


def test_the_cuda_graph_gives_the_same_query_vector_as_the_normal_path(embedder):
    if embedder._graphs is None:
        pytest.skip("no GPU: queries use the normal path")
    queries = [
        "a",
        "resume",
        "binary search time complexity",
        "how do I verify the signature of a refresh token on the server",
    ]
    for query in queries:
        fast = embedder.embed_query(query)
        normal = embedder._encode([query], prompt_name="SearchQuery", batch_size=1)[0]
        assert fast.dtype == np.float32
        np.testing.assert_allclose(np.linalg.norm(fast), 1.0, atol=1e-5)
        assert float(fast @ normal) > 0.9999, query


def test_a_query_too_long_for_any_graph_still_gets_a_vector(embedder):
    query = "word " * 100  # more than 64 tokens
    assert embedder._graphs is None or embedder._graphs.embed(query) is None
    vec = embedder.embed_query(query)
    np.testing.assert_allclose(np.linalg.norm(vec), 1.0, atol=1e-5)
