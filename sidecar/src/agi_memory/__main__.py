"""Process entry point: `uv run python -m agi_memory`.

Contract with the launcher (docs/api.md): the token arrives in `AGI_SIDECAR_TOKEN`, the port in
`AGI_SIDECAR_PORT`; the process binds 127.0.0.1 only, opens the port within a second, reports
`ready: false` on `/health` while the model loads, and is stopped by terminating it.
"""

from __future__ import annotations

import os
import sys

from agi_memory.api.app import MIN_TOKEN_CHARS, create_app
from agi_memory.config import ConfigError, Settings
from agi_memory.embedding.base import Embedder

DEFAULT_PORT = 7861


def _fail(message: str) -> int:
    print(f"agi_memory: {message}", file=sys.stderr)
    return 2


def _load_gemma(settings: Settings) -> Embedder:
    # Imported here so a bad configuration fails fast, without importing torch first.
    from agi_memory.embedding.gemma import GemmaEmbedder

    return GemmaEmbedder(settings)


def main() -> int:
    token = os.environ.get("AGI_SIDECAR_TOKEN", "")
    if len(token) < MIN_TOKEN_CHARS:
        return _fail(f"AGI_SIDECAR_TOKEN must be set to at least {MIN_TOKEN_CHARS} characters")
    port_text = os.environ.get("AGI_SIDECAR_PORT", str(DEFAULT_PORT))
    if not port_text.isdecimal() or not 1 <= int(port_text) <= 65535:
        return _fail(f"AGI_SIDECAR_PORT must be a port number (1-65535), got {port_text!r}")
    try:
        settings = Settings.from_env()
    except ConfigError as error:
        return _fail(str(error))

    import uvicorn

    app = create_app(settings, token, lambda: _load_gemma(settings))
    uvicorn.run(app, host="127.0.0.1", port=int(port_text), log_level="info")
    return 0


if __name__ == "__main__":
    sys.exit(main())
