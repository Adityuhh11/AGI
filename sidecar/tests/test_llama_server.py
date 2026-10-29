import os
import signal
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

import pytest

from omni_memory.embedding import llama_server
from omni_memory.embedding.llama_server import METAL, LlamaServer, ServerError, choose_device

HERE = Path(__file__).parent
FAKE = [sys.executable, str(HERE / "fake_llama_server.py")]


def listing(*lines: str) -> str:
    return "Available devices:\n" + "".join(f"  {line}\n" for line in lines)


def test_the_discrete_gpu_is_chosen_by_name_not_by_memory():
    # the real output of the development laptop: the integrated GPU reports more memory
    assert choose_device((HERE / "data" / "list_devices_vulkan.txt").read_text()) == "Vulkan1"


@pytest.mark.parametrize(
    ("lines", "want"),
    [
        (["CUDA0: NVIDIA GeForce RTX 5070 (12227 MiB, 11000 MiB free)"], "CUDA0"),
        (
            [
                "Vulkan0: AMD Radeon RX 7600 (8176 MiB, 8000 MiB free)",
                "Vulkan1: Intel(R) UHD Graphics (8038 MiB, 7380 MiB free)",
            ],
            "Vulkan0",
        ),
        (
            [
                "Vulkan0: Intel(R) UHD Graphics (8038 MiB, 7380 MiB free)",
                "Vulkan1: NVIDIA RTX A2000 (6144 MiB, 6000 MiB free)",
            ],
            "Vulkan1",
        ),
        (["Vulkan0: Intel(R) Arc(TM) A770 Graphics (16256 MiB, 16000 MiB free)"], "Vulkan0"),
        (["Vulkan0: Intel(R) Iris(R) Xe Graphics (8038 MiB, 7380 MiB free)"], None),
        (["Vulkan0: Intel(R) UHD Graphics (8038 MiB, 7380 MiB free)"], None),
        ([], None),
    ],
)
def test_other_machines(lines, want):
    assert choose_device(listing(*lines)) == want


def test_garbage_listings_choose_nothing():
    assert choose_device("") is None
    assert choose_device("llama-server: error: unknown argument") is None


@pytest.fixture(autouse=True)
def vulkan_machine(request, monkeypatch):
    """The tests below describe a machine with Vulkan; the Mac ones ask for `on_a_mac`."""
    if "on_a_mac" not in request.fixturenames:
        monkeypatch.setattr(llama_server, "_on_a_mac", lambda: False)


@pytest.fixture
def on_a_mac(monkeypatch):
    monkeypatch.setattr(llama_server, "_on_a_mac", lambda: True)


@pytest.fixture
def server(tmp_path, monkeypatch):
    made = []

    def make(mode="ok", **kwargs):
        monkeypatch.setenv("FAKE_LLAMA_MODE", mode)
        s = LlamaServer(FAKE, tmp_path, start_timeout=kwargs.pop("start_timeout", 10), **kwargs)
        made.append(s)
        return s

    yield make
    for s in made:
        s.close()


def health(s: LlamaServer, key: str | None = None) -> int:
    request = urllib.request.Request(
        s.url + "/health", headers={"Authorization": "Bearer " + (key or s.api_key)}
    )
    try:
        with urllib.request.urlopen(request, timeout=3) as response:
            return response.status
    except urllib.error.HTTPError as error:
        return error.code


def test_it_starts_on_the_chosen_gpu_behind_an_api_key(server):
    s = server()
    s.start()
    assert s.device == "vulkan" and s.url.startswith("http://127.0.0.1:")
    assert health(s) == 200 and health(s, "not-the-key") == 401
    assert len(s.api_key) >= 16


def test_it_is_started_with_the_flags_that_were_measured(server, capsys):
    s = server()
    s.start()
    out = capsys.readouterr().err
    for flag in (
        "--embedding",
        "--no-webui",
        "-ub 1024",
        "--image-max-tokens 280",
        "--device Vulkan1",
        "-ngl 99",
        "--host 127.0.0.1",
    ):
        assert flag in out


def test_a_gpu_that_fails_falls_back_to_the_cpu_and_says_so(server):
    s = server("gpu-fails")
    s.start()
    assert s.device == "cpu" and health(s) == 200


def test_the_cpu_can_be_asked_for(server, capsys):
    s = server(cpu=True)
    s.start()
    assert s.device == "cpu"
    assert "--device none" in capsys.readouterr().err


def test_a_server_that_never_starts_is_an_error_with_what_it_printed(server):
    s = server("die")
    with pytest.raises(ServerError, match="failed to initialise the device"):
        s.start()


def test_a_server_that_never_answers_is_given_up_on(server):
    s = server("never-ready", start_timeout=1.5)
    with pytest.raises(ServerError, match="not ready after"):
        s.start()


def test_restart_gives_a_new_process_on_a_new_port_and_the_old_one_is_gone(server):
    s = server()
    s.start()
    old_url, old_key = s.url, s.api_key
    s.restart()
    assert health(s) == 200 and (s.url, s.api_key) != (old_url, old_key)
    with pytest.raises(OSError):
        urllib.request.urlopen(old_url + "/health", timeout=2)


def test_close_stops_the_process_and_may_be_called_twice(server):
    s = server()
    s.start()
    url = s.url
    s.close()
    s.close()
    with pytest.raises(OSError):
        urllib.request.urlopen(url + "/health", timeout=2)


def test_a_missing_program_is_a_clear_error(tmp_path):
    s = LlamaServer(tmp_path / "no-such-llama-server.exe", tmp_path)
    with pytest.raises(ServerError, match="cannot run llama-server"):
        s.start()


CHILD = """
import os, sys
from pathlib import Path
from omni_memory.embedding.llama_server import LlamaServer

server = LlamaServer([sys.argv[1], sys.argv[2]], Path(sys.argv[3]), start_timeout=10)
server.start()
print(server.url, os.getpid(), flush=True)
sys.stdin.read()
"""

# A venv's python.exe is a launcher that starts the real interpreter as a child; a job binds the
# process it is given, so the stand-in server is run by the real one.
REAL_PYTHON = getattr(sys, "_base_executable", sys.executable)


@pytest.mark.skipif(sys.platform != "win32", reason="job objects are Windows")
def test_the_server_dies_when_the_engine_is_killed_without_a_chance_to_clean_up(
    tmp_path, monkeypatch
):
    monkeypatch.setenv("FAKE_LLAMA_MODE", "ok")
    engine = subprocess.Popen(
        [
            sys.executable, "-c", CHILD,
            REAL_PYTHON, str(HERE / "fake_llama_server.py"), str(tmp_path),
        ],
        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True,
    )  # fmt: skip
    assert engine.stdout is not None
    url, pid = engine.stdout.readline().split()
    assert url.startswith("http://127.0.0.1:")
    os.kill(int(pid), signal.SIGTERM)  # TerminateProcess on the real engine: no atexit handler runs
    deadline = time.monotonic() + 10
    try:
        while time.monotonic() < deadline:
            try:
                urllib.request.urlopen(url + "/health", timeout=1)
            except urllib.error.HTTPError:
                time.sleep(0.2)  # a 401 still means the server is alive
            except OSError:
                return
        pytest.fail("the server outlived the engine that was killed")
    finally:
        engine.kill()


def test_a_mac_always_chooses_metal_whatever_the_server_lists(on_a_mac):
    assert choose_device("") == METAL
    assert choose_device(listing("Vulkan1: NVIDIA RTX A2000 (6144 MiB, 6000 MiB free)")) == METAL


def test_on_a_mac_every_layer_goes_to_the_gpu_and_no_device_is_named(on_a_mac, server, capsys):
    s = server()
    s.start()
    out = capsys.readouterr().err
    assert s.device == "metal" and health(s) == 200
    assert "-ngl 99" in out and "--device" not in out
