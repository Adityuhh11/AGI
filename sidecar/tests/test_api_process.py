import json
import os
import socket
import subprocess
import sys
import urllib.error
import urllib.request

import pytest

from helpers import wait_for
from omni_memory.config import EMBEDDING_DIM, MODEL_ID
from omni_memory.store import FlatStore

pytestmark = pytest.mark.model

TOKEN = "p" * 32


def free_port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def call(port, method, path, *, token=TOKEN, body=None):
    request = urllib.request.Request(
        f"http://127.0.0.1:{port}{path}",
        method=method,
        data=None if body is None else json.dumps(body).encode(),
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            return response.status, json.loads(response.read())
    except urllib.error.HTTPError as error:
        return error.code, json.loads(error.read())


def answering(port):
    try:
        return call(port, "GET", "/health")[1]
    except (urllib.error.URLError, ConnectionError, TimeoutError):
        return None


def test_the_real_process_honours_the_start_ready_stop_contract(tmp_path):
    folder = tmp_path / "project"
    (folder / "Backend").mkdir(parents=True)
    (folder / "Backend" / "jwt.js").write_text("function signRefreshToken() { refresh token jwt }")
    (folder / "Backend" / "addcountries.js").write_text("async function addCountries() { seed }")
    (folder / "notes.md").write_text("# Notes\nthe quick brown fox jumps over the lazy dog")
    port = free_port()
    env = {
        **os.environ,
        "OMNI_SIDECAR_TOKEN": TOKEN,
        "OMNI_SIDECAR_PORT": str(port),
        "OMNI_MEMORY_DIR": str(tmp_path / "idx"),
    }
    process = subprocess.Popen(
        [sys.executable, "-m", "omni_memory"],
        env=env,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    try:
        wait_for(lambda: answering(port), timeout=60)  # the port opens before the model is loaded
        health = wait_for(lambda: (h := answering(port)) and h["ready"] and h, timeout=180)
        assert (health["model"], health["error"]) == (MODEL_ID, None)
        assert health["device"] in {"vulkan", "cuda", "cpu"}
        assert call(port, "GET", "/health", token="wrong")[0] == 401

        body = {"path": str(folder), "source_id": "project"}
        status, accepted = call(port, "POST", "/sources", body=body)
        assert status == 202
        job = wait_for(
            lambda: (
                (j := call(port, "GET", f"/jobs/{accepted['job_id']}")[1])["state"] != "running"
                and j
            ),
            timeout=180,
        )
        assert (job["state"], job["report"]["added"]) == ("done", 3)

        status, found = call(
            port, "POST", "/search", body={"query": "how do I refresh the jwt token", "k": 3}
        )
        assert status == 200
        assert found["hits"][0]["uri"] == "Backend/jwt.js"
    finally:
        process.terminate()  # what the Go side does (a Job Object kills it the same way)
        process.wait(timeout=30)
    store = FlatStore(tmp_path / "idx", model_id=MODEL_ID, dim=EMBEDDING_DIM)
    try:
        assert store.count() == 3  # the index survived a hard stop
    finally:
        store.close()
