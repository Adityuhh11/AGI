"""Entry point: `uv run python -m omni_memory.eval` (see `runner.run` for the options)."""

from __future__ import annotations

import datetime
import os
import sys

from omni_memory.config import Settings
from omni_memory.embedding.base import Embedder
from omni_memory.eval.runner import run


def _llama(cpu: bool) -> Embedder:
    # Imported here so unit tests and `--help` never start a server.
    from omni_memory.embedding.llama import LlamaEmbedder
    from omni_memory.embedding.llama_server import LlamaServer

    settings = Settings.from_env({"OMNI_MEMORY_DIR": ".", **os.environ})
    if settings.llama_server is None or settings.model_dir is None:
        sys.exit("set OMNI_LLAMA_SERVER and OMNI_MODEL_DIR to run the eval")
    server = LlamaServer(settings.llama_server, settings.model_dir, cpu=cpu)
    server.start()
    return LlamaEmbedder(server, settings)


if __name__ == "__main__":
    sys.exit(run(sys.argv[1:], _llama, datetime.date.today().isoformat()))
