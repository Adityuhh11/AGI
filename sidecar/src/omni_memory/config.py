"""Settings for the memory engine: one frozen object, built once and passed explicitly."""

from __future__ import annotations

import os
import sys
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path

MODEL_ID = "google/embeddinggemma-2"
EMBEDDING_DIM = 768  # the model's native size: 256 and 512 measured worse (ADR 0002)


def _data_dir(env: Mapping[str, str], mac: bool) -> Path | None:
    """Where the launcher keeps Omni data on this system; None if the environment does not say."""
    if mac:
        home = env.get("HOME")
        return Path(home) / "Library" / "Application Support" / "Omni" if home else None
    local = env.get("LOCALAPPDATA")
    return Path(local) / "Omni" if local else None


class ConfigError(Exception):
    """The environment does not say where to keep the index."""


@dataclass(frozen=True, slots=True)
class Settings:
    """Everything tunable about the engine. Defaults are the values measured in ADR 0001."""

    index_dir: Path
    llama_server: Path | None = None  # the llama.cpp server program, from OMNI_LLAMA_SERVER
    model_dir: Path | None = None  # the folder with the two GGUF files (OMNI_MODEL_DIR)
    model_id: str = MODEL_ID
    dim: int = EMBEDDING_DIM
    max_seq_length: int = 512  # tokens; real chunks are small, and the server rejects longer inputs
    text_batch_size: int = 8
    image_batch_size: int = 4
    audio_batch_size: int = 4  # 30 s windows embedded per call; two requests are in flight
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
        """The llama.cpp server, the models and the index come from `OMNI_LLAMA_SERVER`,
        `OMNI_MODEL_DIR` and `OMNI_MEMORY_DIR` (the launcher sets all three). Started by hand
        without `OMNI_MEMORY_DIR`, the index is where the launcher would keep it."""
        env = os.environ if environ is None else environ
        server, models = env.get("OMNI_LLAMA_SERVER"), env.get("OMNI_MODEL_DIR")
        llama_server = Path(server) if server else None
        model_dir = Path(models) if models else None
        if override := env.get("OMNI_MEMORY_DIR"):
            return cls(index_dir=Path(override), llama_server=llama_server, model_dir=model_dir)
        if base := _data_dir(env, sys.platform == "darwin"):
            return cls(index_dir=base / "memory", llama_server=llama_server, model_dir=model_dir)
        raise ConfigError("cannot locate the index: set OMNI_MEMORY_DIR")
