import os
import subprocess
import sys

import pytest


def run_main(tmp_path, **env_overrides):
    env = {k: v for k, v in os.environ.items() if not k.startswith("OMNI_")}
    env["OMNI_MEMORY_DIR"] = str(tmp_path / "idx")
    env.update(env_overrides)
    return subprocess.run(
        [sys.executable, "-m", "omni_memory"], env=env, capture_output=True, text=True, timeout=60
    )


def test_it_refuses_to_start_without_a_token(tmp_path):
    result = run_main(tmp_path)
    assert result.returncode == 2
    assert "OMNI_SIDECAR_TOKEN" in result.stderr


def test_it_refuses_a_short_token(tmp_path):
    result = run_main(tmp_path, OMNI_SIDECAR_TOKEN="short")
    assert result.returncode == 2
    assert "OMNI_SIDECAR_TOKEN" in result.stderr


@pytest.mark.parametrize("port", ["abc", "0", "70000", "-5", ""])
def test_it_refuses_a_bad_port(tmp_path, port):
    result = run_main(tmp_path, OMNI_SIDECAR_TOKEN="t" * 32, OMNI_SIDECAR_PORT=port)
    assert result.returncode == 2
    assert "OMNI_SIDECAR_PORT" in result.stderr


def test_it_refuses_to_guess_where_the_index_lives(tmp_path):
    env = {k: v for k, v in os.environ.items() if not k.startswith("OMNI_") and k != "LOCALAPPDATA"}
    env["OMNI_SIDECAR_TOKEN"] = "t" * 32
    result = subprocess.run(
        [sys.executable, "-m", "omni_memory"], env=env, capture_output=True, text=True, timeout=60
    )
    assert result.returncode == 2
    assert "OMNI_MEMORY_DIR" in result.stderr
