"""Process entry point: `uv run python -m omni_memory`.

Contract with the launcher (docs/api.md): the token arrives in `OMNI_SIDECAR_TOKEN`, the port in
`OMNI_SIDECAR_PORT`; the process binds 127.0.0.1 only, opens the port within a second, reports
`ready: false` on `/health` while the model loads, and is stopped by terminating it.
"""

from __future__ import annotations

import os
import sys

from omni_memory.api.app import MIN_TOKEN_CHARS, create_app
from omni_memory.config import ConfigError, Settings
from omni_memory.embedding.base import Embedder

DEFAULT_PORT = 7861


def _fail(message: str) -> int:
    print(f"omni_memory: {message}", file=sys.stderr)
    return 2


def _load_embedder(settings: Settings) -> Embedder:
    # Imported here so a bad configuration fails fast.
    from omni_memory.embedding.llama import LlamaEmbedder
    from omni_memory.embedding.llama_server import LlamaServer

    assert settings.llama_server is not None and settings.model_dir is not None  # checked in main()
    server = LlamaServer(settings.llama_server, settings.model_dir)
    server.start()
    return LlamaEmbedder(server, settings)


def _missing_runtime(settings: Settings) -> str | None:
    """Why the embedding runtime cannot start, or None if its files are there."""
    from omni_memory.embedding.llama_server import TEXT_MODEL, VISION_MODEL

    if settings.llama_server is None or settings.model_dir is None:
        return "OMNI_LLAMA_SERVER and OMNI_MODEL_DIR must be set (the launcher sets them)"
    paths = (
        settings.llama_server,
        settings.model_dir / TEXT_MODEL,
        settings.model_dir / VISION_MODEL,
    )
    for path in paths:
        if not path.is_file():
            return f"{path} is missing; turn file search off and on again in Settings"
    return None


def main() -> int:
    token = os.environ.get("OMNI_SIDECAR_TOKEN", "")
    if len(token) < MIN_TOKEN_CHARS:
        return _fail(f"OMNI_SIDECAR_TOKEN must be set to at least {MIN_TOKEN_CHARS} characters")
    port_text = os.environ.get("OMNI_SIDECAR_PORT", str(DEFAULT_PORT))
    if not port_text.isdecimal() or not 1 <= int(port_text) <= 65535:
        return _fail(f"OMNI_SIDECAR_PORT must be a port number (1-65535), got {port_text!r}")
    try:
        settings = Settings.from_env()
    except ConfigError as error:
        return _fail(str(error))

    if problem := _missing_runtime(settings):
        return _fail(problem)

    import uvicorn

    app = create_app(settings, token, lambda: _load_embedder(settings))
    uvicorn.run(
        app, host="127.0.0.1", port=int(port_text), log_level="info", access_log=False
    )  # the launcher polls /health twice a second
    return 0


if __name__ == "__main__":
    sys.exit(main())
