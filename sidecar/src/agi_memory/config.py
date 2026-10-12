"""Settings for the memory engine: one frozen object, built once and passed explicitly."""

from __future__ import annotations

import os
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path

MODEL_ID = "google/embeddinggemma-2"
EMBEDDING_DIM = 256


class ConfigError(Exception):
    """The environment does not say where to keep the index."""


@dataclass(frozen=True, slots=True)
class Settings:
    """Everything tunable about the engine. Defaults are the values measured in ADR 0001."""

    index_dir: Path
    model_id: str = MODEL_ID
    dim: int = EMBEDDING_DIM
    max_seq_length: int = 512  # 8192 x batch overflows 6 GB VRAM; real chunks are small
    text_batch_size: int = 8
    image_batch_size: int = 4

    @classmethod
    def from_env(cls, environ: Mapping[str, str] | None = None) -> Settings:
        """Index location: `AGI_MEMORY_DIR` if set, else `%LOCALAPPDATA%\\AGI\\memory`."""
        env = os.environ if environ is None else environ
        if override := env.get("AGI_MEMORY_DIR"):
            return cls(index_dir=Path(override))
        if local := env.get("LOCALAPPDATA"):
            return cls(index_dir=Path(local) / "AGI" / "memory")
        raise ConfigError("cannot locate the index: set AGI_MEMORY_DIR (LOCALAPPDATA is unset)")
