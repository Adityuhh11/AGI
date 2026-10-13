import pytest

from agi_memory.store import FlatStore
from api_helpers import running_app
from helpers import DIM


@pytest.fixture
def store(tmp_path):
    flat = FlatStore(tmp_path / "idx", model_id="fake", dim=DIM)
    yield flat
    flat.close()


@pytest.fixture
def api(tmp_path):
    with running_app(tmp_path / "idx") as client:
        yield client


@pytest.fixture
def folder(tmp_path):
    root = tmp_path / "docs folder"  # a space on purpose
    (root / "Backend").mkdir(parents=True)
    (root / "Backend" / "jwt.js").write_text("function signRefreshToken() { refresh token jwt }")
    (root / "Backend" / "addcountries.js").write_text("async function addCountries() { seed }")
    (root / "notes.md").write_text("# Notes\nthe quick brown fox jumps over the lazy dog")
    (root / ".env").write_text("JWT_SECRET=hunter2")
    return root
