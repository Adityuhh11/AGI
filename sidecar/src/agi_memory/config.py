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
    max_text_file_bytes: int = 5 * 1024 * 1024
    max_media_file_bytes: int = 50 * 1024 * 1024
    chunk_lines: int = 40
    chunk_overlap_lines: int = 10  # must stay below chunk_lines
    max_chunk_chars: int = 6000  # the model also truncates at max_seq_length tokens
    pdf_render_scale: float = 1.5
    image_max_side: int = 1024
    flush_chunks: int = 64  # pending text chunks that trigger one embedding call across files

    def __post_init__(self) -> None:
        if not 0 <= self.chunk_overlap_lines < self.chunk_lines:
            raise ValueError(
                f"chunk_overlap_lines must be in [0, chunk_lines): "
                f"{self.chunk_overlap_lines} vs {self.chunk_lines}"
            )

    @classmethod
    def from_env(cls, environ: Mapping[str, str] | None = None) -> Settings:
        """Index location: `AGI_MEMORY_DIR` if set, else `%LOCALAPPDATA%\\AGI\\memory`."""
        env = os.environ if environ is None else environ
        if override := env.get("AGI_MEMORY_DIR"):
            return cls(index_dir=Path(override))
        if local := env.get("LOCALAPPDATA"):
            return cls(index_dir=Path(local) / "AGI" / "memory")
        raise ConfigError("cannot locate the index: set AGI_MEMORY_DIR (LOCALAPPDATA is unset)")
