from pathlib import Path

import pytest

from agi_memory.config import ConfigError, Settings


def test_env_override_wins(tmp_path):
    settings = Settings.from_env({"AGI_MEMORY_DIR": str(tmp_path), "LOCALAPPDATA": "C:/ignored"})
    assert settings.index_dir == tmp_path


def test_default_lives_under_localappdata():
    settings = Settings.from_env({"LOCALAPPDATA": "C:/Users/x/AppData/Local"})
    assert settings.index_dir == Path("C:/Users/x/AppData/Local/AGI/memory")


def test_missing_location_is_an_error():
    with pytest.raises(ConfigError, match="AGI_MEMORY_DIR"):
        Settings.from_env({})


def test_defaults_match_the_spec(tmp_path):
    s = Settings(index_dir=tmp_path)
    assert (s.model_id, s.dim, s.max_seq_length, s.text_batch_size, s.image_batch_size) == (
        "google/embeddinggemma-2",
        256,
        512,
        8,
        4,
    )
