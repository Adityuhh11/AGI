"""Entry point: `uv run python -m agi_memory.eval` (see `runner.run` for the options)."""

from __future__ import annotations

import datetime
import sys
from pathlib import Path

from agi_memory.config import Settings
from agi_memory.embedding.base import Embedder
from agi_memory.eval.runner import run


def _gemma(cpu: bool) -> Embedder:
    # Imported here so unit tests and `--help` never pay for importing torch.
    from agi_memory.embedding.gemma import GemmaEmbedder

    return GemmaEmbedder(Settings(index_dir=Path(".")), device="cpu" if cpu else None)


if __name__ == "__main__":
    sys.exit(run(sys.argv[1:], _gemma, datetime.date.today().isoformat()))
