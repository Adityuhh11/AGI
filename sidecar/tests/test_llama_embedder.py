import numpy as np
import pytest
from PIL import Image

from agi_memory.config import Settings
from agi_memory.embedding.llama import LlamaEmbedder
from fake_llama import API_KEY, FakeLlama, Handle, expected_for_ids, expected_for_text


@pytest.fixture
def fake():
    server = FakeLlama()
    yield server
    server.close()


def make(fake, tmp_path, **overrides):
    handle = Handle(fake)
    return LlamaEmbedder(handle, Settings(index_dir=tmp_path, **overrides)), handle


def unit(vec):
    vec = np.asarray(vec, dtype=np.float32)
    return vec / np.linalg.norm(vec)


def test_documents_and_queries_use_the_models_own_prompts(fake, tmp_path):
    embedder, _ = make(fake, tmp_path)
    got = embedder.embed_documents(["hello world"], ["notes.md"])
    assert fake.requests[-1][1]["input"] == ["title: notes.md | text: hello world"]
    assert np.allclose(
        got[0], unit(expected_for_text("title: notes.md | text: hello world")), atol=1e-6
    )
    embedder.embed_documents(["no title"])
    assert fake.requests[-1][1]["input"] == ["title: none | text: no title"]
    embedder.embed_query("jwt refresh")
    assert fake.requests[-1][1]["input"] == ["task: search result | query: jwt refresh"]


def test_vectors_are_unit_length_float32_and_a_mismatch_of_titles_is_refused(fake, tmp_path):
    embedder, _ = make(fake, tmp_path)
    got = embedder.embed_documents(["a", "b", "c"], ["x", "y", "z"])
    assert got.shape == (3, 768) and got.dtype == np.float32
    assert np.allclose(np.linalg.norm(got, axis=1), 1.0, atol=1e-5)
    with pytest.raises(ValueError, match="2 titles for 3 texts"):
        embedder.embed_documents(["a", "b", "c"], ["x", "y"])


def test_nothing_in_gives_an_empty_array_and_no_request(fake, tmp_path):
    embedder, _ = make(fake, tmp_path)
    assert embedder.embed_documents([]).shape == (0, 768)
    assert embedder.embed_images([]).shape == (0, 768)
    assert fake.requests == []


def test_a_long_text_is_sent_as_token_ids_cut_to_the_limit_keeping_the_closing_token(
    fake, tmp_path
):
    embedder, _ = make(fake, tmp_path, max_seq_length=512)
    long_text = " ".join(f"word{i}" for i in range(900))
    short_text = "a short one"
    got = embedder.embed_documents([long_text, short_text, long_text], ["a", "b", "c"])
    sent = [body["input"] for path, body in fake.requests if path == "/v1/embeddings"]
    id_requests = [s for s in sent if isinstance(s[0], list)]
    assert len(id_requests) == 1 and len(id_requests[0]) == 2  # both long texts, together, as ids
    for ids in id_requests[0]:
        assert len(ids) == 512 and ids[0] == 2 and ids[-1] == 1
    # the answers come back in the order the texts were given, whatever mix was sent
    assert np.allclose(got[1], unit(expected_for_text("title: b | text: a short one")), atol=1e-6)
    assert np.allclose(got[0], unit(expected_for_ids(id_requests[0][0])), atol=1e-6)
    assert np.allclose(got[2], unit(expected_for_ids(id_requests[0][1])), atol=1e-6)


def test_a_text_at_the_limit_is_not_cut(fake, tmp_path):
    embedder, _ = make(fake, tmp_path, max_seq_length=512)
    text = " ".join(f"w{i}" for i in range(100))  # over 400 characters, about 100 tokens
    embedder.embed_documents([text], ["t"])
    ids = fake.requests[-1][1]["input"][0]
    # opening token, the four words of the "title: t | text:" prefix, the 100 words, closing token
    assert isinstance(ids, list) and len(ids) == 106


def test_one_connection_serves_many_calls(fake, tmp_path):
    embedder, _ = make(fake, tmp_path)
    for i in range(5):
        embedder.embed_query(f"query {i}")
        embedder.embed_documents([f"doc {i}"], ["t"])
    assert len(fake.client_ports) == 1


@pytest.mark.parametrize(
    ("size", "want"),
    [
        ((1024, 1024), (768, 768)),
        ((285, 438), (624, 960)),
        ((1024, 484), (1152, 528)),
        ((296, 768), (480, 1248)),
    ],
)
def test_images_are_resized_to_what_the_models_own_processor_gives(fake, tmp_path, size, want):
    # the wanted sizes are the ones Gemma's own processor produced for these inputs
    # (docs/eval/2026-10-18-llamacpp-embedder.md)
    embedder, _ = make(fake, tmp_path)
    embedder.embed_images([Image.new("RGB", size, (30, 90, 160))])
    assert fake.image_sizes == [want]
    uri = fake.requests[-1][1]["input"][0]["content"][0]["image_url"]["url"]
    assert uri.startswith("data:image/png;base64,")


def test_images_go_one_per_request_and_come_back_in_order(fake, tmp_path):
    embedder, _ = make(fake, tmp_path)
    images = [Image.new("RGB", (200 + 50 * i, 300), (i * 40, 10, 10)) for i in range(5)]
    got = embedder.embed_images(images)
    assert got.shape == (5, 768)
    assert [len(body["input"]) for _, body in fake.requests] == [1] * 5
    alone = [embedder.embed_images([image])[0] for image in images]
    assert np.allclose(got, np.stack(alone), atol=1e-6)


def test_a_smaller_dimension_keeps_the_first_values_and_renormalises(fake, tmp_path):
    embedder, _ = make(fake, tmp_path, dim=256)
    got = embedder.embed_documents(["x"], ["t"])
    assert got.shape == (1, 256)
    assert np.allclose(np.linalg.norm(got, axis=1), 1.0, atol=1e-5)
    full = unit(expected_for_text("title: t | text: x"))
    assert np.allclose(got[0], unit(full[:256]), atol=1e-6)


def test_the_api_key_is_sent_and_a_refusal_is_an_error_not_a_restart(fake, tmp_path):
    embedder, handle = make(fake, tmp_path)
    handle.api_key = "wrong"
    with pytest.raises(RuntimeError, match="answered 401"):
        embedder.embed_query("x")
    assert handle.restarts == 0
    handle.api_key = API_KEY
    embedder.embed_query("x")


def test_a_connection_the_server_closed_while_idle_is_reopened_not_treated_as_a_dead_server(
    fake, tmp_path
):
    embedder, handle = make(fake, tmp_path)
    embedder.embed_query("warm up")
    fake.drop_next = 1  # llama-server closes idle keep-alive connections after a few seconds
    got = embedder.embed_query("after the pause")
    assert handle.restarts == 0
    assert np.allclose(
        got, unit(expected_for_text("task: search result | query: after the pause")), atol=1e-6
    )


def test_a_server_that_is_gone_is_started_again_and_the_new_key_is_used(fake, tmp_path):
    embedder, handle = make(fake, tmp_path)
    embedder.embed_query("warm up")
    fake.drop_next = 2  # the reconnect fails too: the server is dead
    got = embedder.embed_query("after the crash")
    assert handle.restarts == 1 and handle.api_key != API_KEY
    assert np.allclose(
        got, unit(expected_for_text("task: search result | query: after the crash")), atol=1e-6
    )
    embedder.embed_query("and again")  # the connection made after the restart keeps working


def test_a_server_that_fails_again_after_the_restart_fails_the_call(fake, tmp_path):
    embedder, handle = make(fake, tmp_path)
    fake.drop_next = 3
    with pytest.raises(OSError):
        embedder.embed_query("x")
    assert handle.restarts == 1


def test_the_device_is_the_servers(fake, tmp_path):
    embedder, handle = make(fake, tmp_path)
    handle.device = "cpu"
    assert (
        embedder.device == "cpu"
        and embedder.model_id == "google/embeddinggemma-2"
        and embedder.dim == 768
    )
