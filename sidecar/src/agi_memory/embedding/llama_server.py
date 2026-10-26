"""Runs llama.cpp's `llama-server` for the engine: chooses the GPU, waits for it, restarts it.

The server loads EmbeddingGemma 2 as two GGUF files (the text model and the vision projector) and
answers embedding requests on a loopback port that only this process knows, behind a random API key
(a web page in the user's browser could otherwise call a local server; its CORS policy allows every
origin). The device is chosen by name from `--list-devices`: the largest memory is the wrong test,
because an integrated GPU shares system memory and reports more than the real card. If the
server does
not become healthy on the GPU it is started again on the CPU, and `device` says so.
"""

from __future__ import annotations

import atexit
import ctypes
import logging
import os
import re
import secrets
import socket
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.request
from collections import deque
from collections.abc import Sequence
from pathlib import Path

logger = logging.getLogger(__name__)

TEXT_MODEL = "embeddinggemma-2-Q8_0.gguf"
VISION_MODEL = "mmproj-embeddinggemma-2-Q8_0.gguf"
START_TIMEOUT = 60.0  # seconds to become healthy on one device (measured: 3 s from the OS cache)
_TAIL_LINES = 20

# "  Vulkan1: NVIDIA GeForce RTX 4050 Laptop GPU (5920 MiB, 5152 MiB free)"
_DEVICE_LINE = re.compile(r"^\s*(?P<id>[A-Za-z]+\d+):\s*(?P<name>.+?)\s*\(\d+ MiB")
_DISCRETE = (re.compile(r"nvidia|geforce|quadro|rtx", re.I), re.compile(r"amd|radeon", re.I))
_INTEGRATED = re.compile(r"intel.*(uhd|iris|hd graphics|xe graphics)", re.I)


class ServerError(RuntimeError):
    """llama-server could not be started or kept running; the message carries what it printed."""


def choose_device(listing: str) -> str | None:
    """The id (`Vulkan1`) of the discrete GPU in `--list-devices` output, or None for the CPU."""
    devices = [
        (m["id"], m["name"]) for line in listing.splitlines() if (m := _DEVICE_LINE.match(line))
    ]
    for pattern in _DISCRETE:
        for ident, name in devices:
            if pattern.search(name):
                return ident
    return next((ident for ident, name in devices if not _INTEGRATED.search(name)), None)


def _free_port() -> int:
    with socket.socket() as probe:
        probe.bind(("127.0.0.1", 0))
        return probe.getsockname()[1]


class LlamaServer:
    """One llama-server child process. `url`, `api_key` and `device` describe the running one."""

    def __init__(
        self,
        exe: Path | Sequence[str],
        model_dir: Path,
        *,
        cpu: bool = False,
        start_timeout: float = START_TIMEOUT,
    ) -> None:
        # A sequence lets tests run a stand-in script through the interpreter.
        self._exe = [str(exe)] if isinstance(exe, Path) else list(exe)
        self._model_dir = model_dir
        self._start_timeout = start_timeout
        self._cpu = cpu  # skip the GPU (the eval's CPU mode)
        self._process: subprocess.Popen[str] | None = None
        self._tail: deque[str] = deque(maxlen=_TAIL_LINES)
        self._device_id: str | None = None
        self.url = ""
        self.api_key = ""
        self.device = "cpu"
        atexit.register(self.close)

    def start(self) -> None:
        """Choose the device and start the server on it, falling back to the CPU."""
        self._device_id = None if self._cpu else choose_device(self._list_devices())
        self._launch_with_fallback()

    def restart(self) -> None:
        """Start the server again on the device that worked (after it died)."""
        self.close()
        self._launch_with_fallback()

    def fall_back_to_cpu(self) -> None:
        """Start the server again without the graphics card, for good."""
        self._cpu, self._device_id = True, None
        self.restart()

    def close(self) -> None:
        process, self._process = self._process, None
        if process is None or process.poll() is not None:
            return
        process.terminate()
        try:
            process.wait(5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()

    def _list_devices(self) -> str:
        try:
            done = subprocess.run(
                [*self._exe, "--list-devices"],
                capture_output=True,
                text=True,
                timeout=30,
                creationflags=_no_window(),
            )
        except (OSError, subprocess.TimeoutExpired) as error:
            raise ServerError(f"cannot run llama-server: {error}") from error
        return done.stdout + done.stderr

    def _launch_with_fallback(self) -> None:
        attempts = [self._device_id, None] if self._device_id else [None]
        last: ServerError | None = None
        for device_id in attempts:
            try:
                self._launch(device_id)
                return
            except ServerError as error:
                last = error
                self.close()
                if device_id:
                    logger.warning(
                        "llama-server did not start on %s; trying the CPU: %s", device_id, error
                    )
        assert last is not None
        raise last

    def _launch(self, device_id: str | None) -> None:
        port = _free_port()
        self.api_key = secrets.token_hex(16)
        self.url = f"http://127.0.0.1:{port}"
        offload = (
            ["-ngl", "99", "--device", device_id]
            if device_id
            else ["-ngl", "0", "--device", "none"]
        )
        command = [
            *self._exe,
            "-m",
            str(self._model_dir / TEXT_MODEL),
            "--mmproj",
            str(self._model_dir / VISION_MODEL),
            "--embedding",
            "--host",
            "127.0.0.1",
            "--port",
            str(port),
            "--api-key",
            self.api_key,
            "--no-webui",
            # The model's own processor budgets 280 image tokens, which needs a physical batch above
            # 512 (the server caps image tokens by it): docs/eval/2026-10-18-llamacpp-embedder.md.
            "-c",
            "8192",
            "-ub",
            "1024",
            "-b",
            "1024",
            "--image-max-tokens",
            "280",
            "-lv",
            "2",
            "--log-colors",
            "off",
            *offload,
        ]
        self._tail.clear()
        try:
            self._process = subprocess.Popen(
                command,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
                errors="replace",
                creationflags=_no_window(),
            )
        except OSError as error:
            raise ServerError(f"cannot start llama-server: {error}") from error
        _die_with_this_process(self._process)
        threading.Thread(
            target=self._drain, args=(self._process,), name="llama-output", daemon=True
        ).start()
        self._wait_until_healthy()
        self.device = "cpu" if device_id is None else re.sub(r"\d+$", "", device_id).lower()
        logger.info("llama-server is ready on %s (%s)", self.device, device_id or "no GPU")

    def _drain(self, process: subprocess.Popen[str]) -> None:
        """Pass the output on (the launcher logs it) and remember the last lines."""
        assert process.stdout is not None
        for line in process.stdout:
            line = line.rstrip()
            self._tail.append(line)
            print(f"llama-server: {line}", file=sys.stderr, flush=True)

    def _wait_until_healthy(self) -> None:
        deadline = time.monotonic() + self._start_timeout
        request = urllib.request.Request(
            self.url + "/health", headers={"Authorization": "Bearer " + self.api_key}
        )
        while time.monotonic() < deadline:
            process = self._process
            if process is None or process.poll() is not None:
                raise ServerError("llama-server stopped while starting: " + " | ".join(self._tail))
            try:
                with urllib.request.urlopen(request, timeout=2) as response:
                    if response.status == 200:
                        return
            except (urllib.error.URLError, OSError):
                pass
            time.sleep(0.1)
        raise ServerError(
            f"llama-server was not ready after {self._start_timeout:.0f} s: "
            + " | ".join(self._tail)
        )


def _no_window() -> int:
    """Keeps a console window from flashing up on Windows."""
    return subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0


_job: int | None = None


def _die_with_this_process(process: subprocess.Popen[str]) -> None:
    """Puts the server in a Windows job that kills it when this process ends, however it ends.

    The launcher stops the engine with TerminateProcess, which runs no exit handler, so without this
    a server would be left holding a gigabyte of memory and of video memory. A job inside the
    launcher's own job is allowed (Windows 8 and later).
    """
    global _job
    if os.name != "nt":
        return
    from ctypes import wintypes

    class Basic(ctypes.Structure):
        _fields_ = [
            ("PerProcessUserTimeLimit", ctypes.c_int64), ("PerJobUserTimeLimit", ctypes.c_int64),
            ("LimitFlags", wintypes.DWORD), ("MinimumWorkingSetSize", ctypes.c_size_t),
            ("MaximumWorkingSetSize", ctypes.c_size_t), ("ActiveProcessLimit", wintypes.DWORD),
            ("Affinity", ctypes.c_size_t), ("PriorityClass", wintypes.DWORD),
            ("SchedulingClass", wintypes.DWORD),
        ]  # fmt: skip

    class Extended(ctypes.Structure):
        _fields_ = [
            ("BasicLimitInformation", Basic), ("IoInfo", ctypes.c_uint64 * 6),
            ("ProcessMemoryLimit", ctypes.c_size_t), ("JobMemoryLimit", ctypes.c_size_t),
            ("PeakProcessMemoryUsed", ctypes.c_size_t), ("PeakJobMemoryUsed", ctypes.c_size_t),
        ]  # fmt: skip

    kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)
    kernel32.CreateJobObjectW.restype = wintypes.HANDLE
    kernel32.AssignProcessToJobObject.argtypes = [wintypes.HANDLE, wintypes.HANDLE]
    if _job is None:  # one job for the life of this process; it closing kills the servers
        handle = kernel32.CreateJobObjectW(None, None)
        info = Extended()
        info.BasicLimitInformation.LimitFlags = 0x2000  # JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
        extended = 9  # JobObjectExtendedLimitInformation
        ok = kernel32.SetInformationJobObject(
            handle, extended, ctypes.byref(info), ctypes.sizeof(info)
        )
        if not handle or not ok:
            logger.warning("cannot create a job object; the server may outlive the engine")
            return
        _job = handle
    process_handle = int(process._handle)  # type: ignore[attr-defined]  # pyright: ignore[reportAttributeAccessIssue]
    if not kernel32.AssignProcessToJobObject(_job, process_handle):
        logger.warning(
            "cannot bind llama-server to the engine's lifetime: error %s", ctypes.get_last_error()
        )
