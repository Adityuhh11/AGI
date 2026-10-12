import numpy as np
import pytest

from agi_memory.store.vectors import VectorFile, VectorFileError


def rows(*vectors) -> np.ndarray:
    return np.array(vectors, dtype=np.float32)


def test_roundtrip_survives_reopen(tmp_path):
    path = tmp_path / "v.f32"
    vf = VectorFile(path, dim=3, initial_capacity=4)
    vf.write_rows([0, 2], rows([1, 0, 0], [0, 1, 0]))
    vf.flush()
    vf.close()

    vf = VectorFile(path, dim=3)
    assert vf.capacity == 4
    np.testing.assert_array_equal(vf.read(2), [0, 1, 0])
    vf.close()


def test_grows_by_doubling_and_keeps_data(tmp_path):
    vf = VectorFile(tmp_path / "v.f32", dim=2, initial_capacity=2)
    vf.write_rows([0], rows([1, 2]))
    vf.write_rows([5], rows([3, 4]))  # needs 6 rows: 2 -> 4 -> 8
    assert vf.capacity == 8
    np.testing.assert_array_equal(vf.read(0), [1, 2])
    np.testing.assert_array_equal(vf.read(5), [3, 4])
    vf.close()


def test_scores_are_dot_products(tmp_path):
    vf = VectorFile(tmp_path / "v.f32", dim=2, initial_capacity=4)
    vf.write_rows([0, 1, 2], rows([1, 0], [0, 1], [1, 1]))
    result = vf.scores(np.array([1, 0], dtype=np.float32), 3)
    np.testing.assert_array_equal(result, [1, 0, 1])
    vf.close()


def test_scores_stay_valid_after_the_file_grows(tmp_path):
    vf = VectorFile(tmp_path / "v.f32", dim=2, initial_capacity=2)
    vf.write_rows([0, 1], rows([1, 0], [0, 1]))
    result = vf.scores(np.array([1, 0], dtype=np.float32), 2)
    vf.ensure_capacity(1000)
    np.testing.assert_array_equal(result, [1, 0])
    vf.close()


def test_rejects_wrong_dimension(tmp_path):
    path = tmp_path / "v.f32"
    VectorFile(path, dim=3).close()
    with pytest.raises(VectorFileError, match="dim"):
        VectorFile(path, dim=4)


def test_rejects_foreign_file(tmp_path):
    path = tmp_path / "v.f32"
    path.write_bytes(b"x" * 200)
    with pytest.raises(VectorFileError, match="not a vector file"):
        VectorFile(path, dim=3)


def test_rejects_empty_file(tmp_path):
    path = tmp_path / "v.f32"
    path.write_bytes(b"")
    with pytest.raises(VectorFileError, match="truncated"):
        VectorFile(path, dim=3)


def test_rejects_truncated_file(tmp_path):
    path = tmp_path / "v.f32"
    VectorFile(path, dim=3, initial_capacity=4).close()  # 64 + 4*3*4 = 112 bytes
    path.write_bytes(path.read_bytes()[:100])
    with pytest.raises(VectorFileError, match="shorter"):
        VectorFile(path, dim=3)


def test_rejects_bad_shapes_and_capacity(tmp_path):
    vf = VectorFile(tmp_path / "v.f32", dim=3)
    with pytest.raises(ValueError, match="shape"):
        vf.write_rows([0], rows([1, 2]))
    vf.close()
    with pytest.raises(ValueError, match="initial_capacity"):
        VectorFile(tmp_path / "other.f32", dim=3, initial_capacity=0)


def test_close_releases_the_file_and_is_idempotent(tmp_path):
    path = tmp_path / "v.f32"
    vf = VectorFile(path, dim=3)
    vf.write_rows([0], rows([1, 0, 0]))
    vf.close()
    vf.close()
    path.unlink()  # raises PermissionError on Windows if a handle or mapping leaked
