import pytest

from omni_memory.config import ConfigError, Settings


def test_env_override_wins(tmp_path):
    settings = Settings.from_env({"OMNI_MEMORY_DIR": str(tmp_path)})
    assert settings.index_dir == tmp_path


def test_a_system_data_folder_is_not_guessed():
    with pytest.raises(ConfigError, match="OMNI_MEMORY_DIR"):
        Settings.from_env({"LOCALAPPDATA": "C:/Users/x/AppData/Local", "HOME": "/Users/x"})


def test_missing_location_is_an_error():
    with pytest.raises(ConfigError, match="OMNI_MEMORY_DIR"):
        Settings.from_env({})


def test_defaults_match_the_spec(tmp_path):
    s = Settings(index_dir=tmp_path)
    assert (s.model_id, s.dim, s.max_seq_length, s.text_batch_size, s.image_batch_size) == (
        "google/embeddinggemma-2",
        768,
        512,
        8,
        4,
    )


def test_limits_match_the_spec(tmp_path):
    s = Settings(index_dir=tmp_path)
    assert (s.max_text_file_bytes, s.max_media_file_bytes) == (5 * 1024 * 1024, 50 * 1024 * 1024)
    assert (s.chunk_lines, s.chunk_overlap_lines, s.max_chunk_chars) == (40, 10, 6000)
    assert (s.pdf_render_scale, s.image_max_side, s.flush_chunks) == (1.5, 1024, 64)


@pytest.mark.parametrize("overlap", [-1, 40, 41])
def test_overlap_must_be_smaller_than_the_window(tmp_path, overlap):
    with pytest.raises(ValueError, match="chunk_overlap_lines"):
        Settings(index_dir=tmp_path, chunk_lines=40, chunk_overlap_lines=overlap)
