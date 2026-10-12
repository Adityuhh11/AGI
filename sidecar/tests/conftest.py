import pytest

from agi_memory.store import FlatStore
from helpers import DIM


@pytest.fixture
def store(tmp_path):
    flat = FlatStore(tmp_path / "idx", model_id="fake", dim=DIM)
    yield flat
    flat.close()
