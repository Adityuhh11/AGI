# Memory Engine, Phases 0-2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the foundation of the sidecar: package, tooling and docs (phase 0), the embedding contract with a deterministic fake and the real EmbeddingGemma 2 (phase 1), and the persistent exact-search store (phase 2).

**Architecture:** A Python package `omni_memory` under `sidecar/src/`. `Embedder` turns text and images into 256-d unit vectors (real model on the GPU, fake hashed model for tests). `FlatStore` keeps chunk metadata and a keyword index in one SQLite file and vectors in one memory-mapped float32 file, and answers exact top-k queries on the CPU.

**Tech Stack:** Python >= 3.12, uv, numpy, sqlite3 (FTS5), sentence-transformers 6.1 + torch (CUDA build) for the model, pytest, ruff, pyright.

**Spec:** `docs/superpowers/specs/2026-10-12-memory-engine-design.md` (phases 0, 1, 2 of section 13). Decision record: `docs/decisions/0001-vector-store.md`. Later phases (ingestion, search pipeline and eval, API) get their own plans once these phases show how the model API and store behave in practice.

## Global Constraints

- Python `>=3.12`; environment managed by `uv`; run all commands from `C:\dev\VS_CODE\Omni\sidecar` unless a step says otherwise.
- Windows 11, RTX 4050 with 6 GB VRAM. The GPU hosts **only the embedding model**; index storage and search run on the CPU.
- Embedding model id `google/embeddinggemma-2`, text + vision (no audio), vectors truncated to **256** dimensions and L2-normalised, `max_seq_length` 512, text batch size 8, image batch size 4.
- Vector file stores **float32** rows; ids are content-derived; ranked output ties are broken by chunk id.
- Index directory `%LOCALAPPDATA%\Omni\memory\`, override with env `OMNI_MEMORY_DIR` (tests always use a temp directory).
- Determinism: tests never need the GPU or the real model unless marked `model`; no unseeded randomness or hidden clocks in logic code.
- Quality gate for every task: `uv run ruff check .`, `uv run ruff format --check .`, `uv run pyright`, `uv run pytest` all clean before commit.
- Conventional commits, one logical change per commit, tests in the same commit as the code they cover, docs updated in the same commit as the behaviour they describe. End every commit message with the `Co-Authored-By` trailer the harness specifies.
- Never read, index, log or commit `.env*` files or anything under `Sample Data/` (it is git-ignored).

## Review Focus

Inputs the spec implies but its stated tests do not exercise; each has a test in the owning task.

1. Search on an empty store, or with `n` larger than the number of live chunks, returns `[]` / all live chunks and never crashes (Task 9).
2. The same chunk id twice in one batch is rejected; re-upserting an existing id keeps one chunk with the latest vector (Tasks 8, 9).
3. Bad vectors (wrong dimension, float64, NaN, not unit length) are rejected before anything touches disk (Task 8).
4. Hostile chunk text (quotes, `*`, `NEAR(`, emoji, CRLF) is stored verbatim and the keyword index stays consistent (Task 8).
5. Windows file locking: a closed store leaves no open handles (directory removable), and the vector file growing while another thread searches does not crash (Tasks 7, 8, 9).

## File Structure

```
sidecar/
  pyproject.toml                       (modify)
  src/omni_memory/
    __init__.py                        package docstring
    types.py                           Chunk, ChunkKind, Filters, VectorHit, chunk_id
    config.py                          Settings, ConfigError
    embedding/
      __init__.py                      exports Embedder, FakeEmbedder (not GemmaEmbedder: it imports torch)
      base.py                          Embedder protocol
      fake.py                          FakeEmbedder (hashed bag of words)
      gemma.py                         GemmaEmbedder (real model)
    store/
      __init__.py                      exports FlatStore, IndexMismatchError
      identifiers.py                   split_identifiers
      vectors.py                       VectorFile (memory-mapped float32 matrix)
      schema.py                        SQL for meta.db version 1
      flat.py                          FlatStore
  tests/
    conftest.py                        `store` fixture
    helpers.py                         make_chunk, unit_vectors, DIM
    test_smoke.py  test_types.py  test_config.py  test_fake_embedder.py  test_gemma_embedder.py
    test_identifiers.py  test_vector_file.py  test_flat_store.py  test_flat_search.py  test_flat_crash.py
docs/README.md  docs/ARCHITECTURE.md  docs/STATUS.md   (create)
AGENTS.md                              (modify)
```

Deviations from the spec found while planning (the plan fixes the spec text in the task named):

- Chunk id also hashes `source_id`, otherwise identical files in two sources collide (Task 3).
- SQLite column is `vec_row` (not `row`, a SQLite keyword) and `chunks` has an explicit integer primary key `rid` so the external-content FTS mapping survives `VACUUM` (Task 8).
- The `sources` and `files` tables are created by the ingestion plan (phase 3), not here; the `Store` protocol is extracted when the benchmark harness needs a second implementation (phase 4).

---

## Phase 0: Foundations

### Task 1: Package skeleton and quality tooling

**Files:**
- Modify: `sidecar/pyproject.toml`
- Modify: `.gitignore`
- Create: `sidecar/src/omni_memory/__init__.py`
- Create: `sidecar/tests/test_smoke.py`

**Interfaces:**
- Produces: importable package `omni_memory`; commands `uv run pytest`, `uv run ruff check .`, `uv run ruff format --check .`, `uv run pyright`; pytest marker `model` (excluded by default, run with `-m model`).

- [ ] **Step 1: Write the failing test**

Create `sidecar/tests/test_smoke.py`:

```python
def test_package_imports_and_documents_itself():
    import omni_memory

    assert omni_memory.__doc__
```

- [ ] **Step 2: Run it to see it fail**

Run: `uv run pytest tests/test_smoke.py -v`
Expected: FAIL (`ModuleNotFoundError: No module named 'omni_memory'`).

- [ ] **Step 3: Replace `sidecar/pyproject.toml`**

```toml
[project]
name = "omni-sidecar"
version = "0.1.0"
description = "Local memory engine for Omni: indexing and search over localhost HTTP."
requires-python = ">=3.12"
dependencies = [
    "numpy",
    "pillow",
    "sentence-transformers[image]>=6.1.0",
    "torch",
    "transformers",
]

[dependency-groups]
dev = ["pytest", "ruff", "pyright"]

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[tool.hatch.build.targets.wheel]
packages = ["src/omni_memory"]

# Plain PyPI ships CPU-only torch on Windows; pull the CUDA build instead.
[tool.uv.sources]
torch = { index = "pytorch-cu128" }

[[tool.uv.index]]
name = "pytorch-cu128"
url = "https://download.pytorch.org/whl/cu128"
explicit = true

[tool.ruff]
line-length = 100
src = ["src", "tests"]

[tool.ruff.lint]
select = ["E", "F", "I", "UP", "B", "SIM", "RUF", "D100", "D104"]

[tool.ruff.lint.per-file-ignores]
"tests/**" = ["D"]

[tool.pytest.ini_options]
testpaths = ["tests"]
addopts = "-ra --strict-markers -m 'not model'"
markers = ["model: needs the real embedding model (and a GPU); run with -m model"]

[tool.pyright]
include = ["src"]
pythonVersion = "3.12"
typeCheckingMode = "standard"
```

- [ ] **Step 4: Create the package and ignore tool caches**

Create `sidecar/src/omni_memory/__init__.py`:

```python
"""Omni memory engine: indexes the owner's files and answers 'find the thing I mean' queries."""
```

Append to the repo-root `.gitignore`:

```
.pytest_cache/
.ruff_cache/
*.egg-info/
```

- [ ] **Step 5: Sync, then run the full gate**

Run: `uv sync`
Run: `uv run pytest -v && uv run ruff check . && uv run ruff format --check . && uv run pyright`
Expected: 1 test passed; ruff and pyright report no problems (pyright prints `0 errors`). If `ruff format --check` complains, run `uv run ruff format .` and re-run.

- [ ] **Step 6: Commit**

```bash
git add .gitignore sidecar/pyproject.toml sidecar/uv.lock sidecar/src sidecar/tests
git commit -m "chore(sidecar): package layout and quality tooling"
```

### Task 2: Documentation scaffold

**Files:**
- Create: `docs/README.md`, `docs/ARCHITECTURE.md`, `docs/STATUS.md`
- Modify: `AGENTS.md` (the `## Docs` section)

**Interfaces:** Produces the three living docs that every later task updates.

- [ ] **Step 1: Create `docs/README.md`**

```markdown
# Documentation index

Start with `STATUS.md`, then `ARCHITECTURE.md`.

| Doc | Answers |
|---|---|
| `STATUS.md` | What is built, what is next, known gaps. Read first. |
| `ARCHITECTURE.md` | How the system fits together: modules, data flow, on-disk formats, invariants. |
| `decisions/` | Decision records: what we chose, the alternatives, the numbers behind it. |
| `superpowers/specs/` | Design specs: what we are building and why. |
| `superpowers/plans/` | Implementation plans: how, task by task. |
| `eval/` | Retrieval-quality reports and the baseline (created in phase 4). |

Rules for keeping docs honest live in `../AGENTS.md`: update the doc in the same commit as the code.
```

- [ ] **Step 2: Create `docs/ARCHITECTURE.md`**

````markdown
# Architecture

Living document. Update it in the same commit as the code it describes.
Spec: `superpowers/specs/2026-10-12-memory-engine-design.md`.

## Overview

Omni is a local-first, AI-native launcher for Windows. Two processes cooperate:

- the **Go launcher** (later): hotkey, UI (Wails v3 + Svelte), calls the sidecar;
- the **Python sidecar** (`sidecar/`): the *memory engine* that indexes files and answers search queries.

They talk over authenticated localhost HTTP. The GPU hosts only the embedding model; index storage and search run on the CPU.

## Memory engine modules (`sidecar/src/omni_memory/`)

Dependencies point downward only. Status is `planned` or `built`.

| Module | Job | Status |
|---|---|---|
| `types.py` | Plain data: `Chunk`, `ChunkKind`, `Filters`, `VectorHit`, `chunk_id` | planned |
| `config.py` | `Settings`: paths, model id, dimensions, batch sizes | planned |
| `embedding/` | `Embedder` contract; `GemmaEmbedder` (real), `FakeEmbedder` (tests) | planned |
| `store/` | `FlatStore`: metadata + keyword index (SQLite) and exact vector search (memory-mapped file) | planned |
| `chunking/`, `sources/`, `jobs.py` | Turn files into chunks and index them incrementally | phase 3 |
| `search/` | Vector + keyword search, rank fusion | phase 4 |
| `eval/` | Golden queries, metrics, baseline | phase 4 |
| `api/` | Authenticated localhost HTTP API | phase 5 |

## Data flow

Indexing: source file -> chunks -> embeddings (256-d unit vectors) -> `FlatStore.upsert`.
Search: query -> embedding + keyword query -> two ranked lists -> fusion -> hits.

## On-disk format

Filled in as each piece is built (see the store section after phase 2).

## Invariants

1. **Determinism.** Chunk ids are content-derived; every ranked list breaks ties by chunk id; tests never depend on the GPU or the real model unless marked `model`.
2. **Privacy.** Nothing leaves the machine. `.env*`, keys and similar files are never read (deny list, phase 3).
3. **SQLite is the source of truth** for what exists in the index; vector rows without a metadata entry are garbage.
4. **GPU = models only.** Index and search live on the CPU.

## Conventions

- One module = one job; docstrings say *why*; public behaviour has tests.
- Errors carry context and are never swallowed.
- Commits are small, conventional (`feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `test:`).
````

- [ ] **Step 3: Create `docs/STATUS.md`**

```markdown
# Status

Updated at the end of every phase. Last updated: 2026-10-12.

| Phase | Scope | State |
|---|---|---|
| 0 Foundations | Package, tooling, settings, docs scaffold | in progress |
| 1 Embedding | `Embedder`, `FakeEmbedder`, `GemmaEmbedder` | not started |
| 2 Store | `FlatStore`: metadata, FTS, exact search, crash safety | not started |
| 3 Ingestion | Ignore rules, chunkers, incremental sync, jobs | not planned yet |
| 4 Search + eval | Keyword + vector fusion, eval harness, baseline | not planned yet |
| 5 API | Authenticated FastAPI app | not planned yet |
| 6 Quality iteration | tree-sitter chunking, rerankers, "no good answer" | ideas only |

## Next

Execute `superpowers/plans/2026-10-12-memory-engine-phases-0-2.md`.

## Known gaps and deviations from the spec

- Chunk id also hashes `source_id` (spec said uri + locator + content hash).
- SQLite column is `vec_row`; `chunks` has an integer primary key `rid` (VACUUM-safe FTS mapping).
- `sources`/`files` tables arrive with ingestion (phase 3); the `Store` protocol is extracted in phase 4.
```

- [ ] **Step 4: Point `AGENTS.md` at them**

In `AGENTS.md`, replace the line `## Docs` (followed by a blank line) with:

```markdown
## Docs

Start here: `docs/README.md` (index), then `docs/STATUS.md` (what is built, what is next) and
`docs/ARCHITECTURE.md` (how it fits together).

```

- [ ] **Step 5: Commit**

```bash
git add docs/README.md docs/ARCHITECTURE.md docs/STATUS.md AGENTS.md
git commit -m "docs: add docs index, architecture and status scaffold"
```

### Task 3: Shared types and settings

**Files:**
- Create: `sidecar/src/omni_memory/types.py`, `sidecar/src/omni_memory/config.py`
- Test: `sidecar/tests/test_types.py`, `sidecar/tests/test_config.py`
- Modify: `docs/superpowers/specs/2026-10-12-memory-engine-design.md` (section 5, chunk id), `docs/ARCHITECTURE.md`, `docs/STATUS.md`

**Interfaces:**
- Produces (`omni_memory.types`):
  - `ChunkKind` (`enum.StrEnum`): `CODE, MARKDOWN, TEXT, PDF_TEXT, PDF_PAGE, IMAGE` with values `"code","markdown","text","pdf_text","pdf_page","image"`.
  - `chunk_id(source_id: str, uri: str, locator: str, content_hash: str) -> str` (sha256 hex).
  - `Chunk(source_id, uri, kind, locator, content_hash, mtime_ns, text)`: frozen; property `.id -> str`.
  - `Filters(kinds: frozenset[ChunkKind] | None = None, source_id: str | None = None, uri_prefix: str | None = None, modified_after_ns: int | None = None)`: frozen.
  - `VectorHit(id: str, score: float)`: frozen.
- Produces (`omni_memory.config`): `ConfigError`; `Settings(index_dir: Path, model_id: str = "google/embeddinggemma-2", dim: int = 256, max_seq_length: int = 512, text_batch_size: int = 8, image_batch_size: int = 4)`; `Settings.from_env(environ: Mapping[str, str] | None = None) -> Settings`.

- [ ] **Step 1: Write the failing tests**

`sidecar/tests/test_types.py`:

```python
import dataclasses

import pytest

from omni_memory.types import Chunk, ChunkKind, chunk_id


def make(**overrides) -> Chunk:
    fields = dict(
        source_id="repo",
        uri="src/a.js",
        kind=ChunkKind.CODE,
        locator='{"lines":[1,40]}',
        content_hash="abc",
        mtime_ns=1,
        text="hello",
    )
    return Chunk(**{**fields, **overrides})


def test_id_is_deterministic():
    assert make().id == make().id
    assert make().id == chunk_id("repo", "src/a.js", '{"lines":[1,40]}', "abc")


def test_id_ignores_fields_that_are_not_identity():
    assert make(mtime_ns=999, text="different").id == make().id


@pytest.mark.parametrize(
    "override",
    [
        {"source_id": "other"},
        {"uri": "src/b.js"},
        {"locator": '{"lines":[31,70]}'},
        {"content_hash": "changed"},
    ],
)
def test_id_changes_with_identity_fields(override):
    assert make(**override).id != make().id


def test_chunk_is_immutable():
    with pytest.raises(dataclasses.FrozenInstanceError):
        make().text = "nope"  # type: ignore[misc]
```

`sidecar/tests/test_config.py`:

```python
from pathlib import Path

import pytest

from omni_memory.config import ConfigError, Settings


def test_env_override_wins(tmp_path):
    settings = Settings.from_env({"OMNI_MEMORY_DIR": str(tmp_path), "LOCALAPPDATA": "C:/ignored"})
    assert settings.index_dir == tmp_path


def test_default_lives_under_localappdata():
    settings = Settings.from_env({"LOCALAPPDATA": "C:/Users/x/AppData/Local"})
    assert settings.index_dir == Path("C:/Users/x/AppData/Local/Omni/memory")


def test_missing_location_is_an_error():
    with pytest.raises(ConfigError, match="OMNI_MEMORY_DIR"):
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
```

- [ ] **Step 2: Run them to see them fail**

Run: `uv run pytest tests/test_types.py tests/test_config.py -v`
Expected: FAIL (`ModuleNotFoundError: omni_memory.types` / `omni_memory.config`).

- [ ] **Step 3: Implement `sidecar/src/omni_memory/types.py`**

```python
"""Plain data types shared across the memory engine: no I/O, no behaviour beyond identity."""

from __future__ import annotations

import enum
import hashlib
from dataclasses import dataclass


class ChunkKind(enum.StrEnum):
    """What a chunk's content is; decides how it is embedded and how it is shown."""

    CODE = "code"
    MARKDOWN = "markdown"
    TEXT = "text"
    PDF_TEXT = "pdf_text"
    PDF_PAGE = "pdf_page"
    IMAGE = "image"


def chunk_id(source_id: str, uri: str, locator: str, content_hash: str) -> str:
    """Content-derived id, so re-indexing the same content always yields the same id.

    `source_id` is part of the identity: two sources can hold identical files at the same
    relative path and must not collide.
    """
    return hashlib.sha256(f"{source_id}\0{uri}\0{locator}\0{content_hash}".encode()).hexdigest()


@dataclass(frozen=True, slots=True)
class Chunk:
    """One searchable unit of a source file. Metadata only; vectors live in the store."""

    source_id: str
    uri: str  # path relative to the source root, '/'-separated
    kind: ChunkKind
    locator: str  # JSON text locating the chunk, e.g. '{"lines":[1,40]}' or '{"page":3}'
    content_hash: str  # sha256 hex of the chunk's content
    mtime_ns: int
    text: str  # text indexed for keyword search; '' for images

    @property
    def id(self) -> str:
        return chunk_id(self.source_id, self.uri, self.locator, self.content_hash)


@dataclass(frozen=True, slots=True)
class Filters:
    """Restrictions applied before ranking. `None` means no restriction on that field."""

    kinds: frozenset[ChunkKind] | None = None
    source_id: str | None = None
    uri_prefix: str | None = None
    modified_after_ns: int | None = None


@dataclass(frozen=True, slots=True)
class VectorHit:
    """One result of an exact vector search."""

    id: str
    score: float  # cosine similarity, because stored rows are unit length
```

- [ ] **Step 4: Implement `sidecar/src/omni_memory/config.py`**

```python
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
        """Index location: `OMNI_MEMORY_DIR` if set, else `%LOCALAPPDATA%\\Omni\\memory`."""
        env = os.environ if environ is None else environ
        if override := env.get("OMNI_MEMORY_DIR"):
            return cls(index_dir=Path(override))
        if local := env.get("LOCALAPPDATA"):
            return cls(index_dir=Path(local) / "Omni" / "memory")
        raise ConfigError("cannot locate the index: set OMNI_MEMORY_DIR (LOCALAPPDATA is unset)")
```

- [ ] **Step 5: Fix the spec's chunk id**

In `docs/superpowers/specs/2026-10-12-memory-engine-design.md` section 5, replace the sentence ``` `id` = sha256 of `uri + locator + content_hash` (hex). ``` with ``` `id` = sha256 of `source_id + uri + locator + content_hash` (hex, `\0`-separated). ```

- [ ] **Step 6: Run the gate**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run pyright && uv run pytest -v`
Expected: 12 tests pass (1 smoke, 7 types, 4 config), ruff and pyright clean.

- [ ] **Step 7: Phase 0 docs gate**

In `docs/ARCHITECTURE.md` change the Status of the `types.py` and `config.py` rows to `built`. In `docs/STATUS.md` set phase 0 to `done` and update the "Last updated" date.

- [ ] **Step 8: Commit**

```bash
git add sidecar/src sidecar/tests docs
git commit -m "feat(sidecar): shared types and settings; fix chunk id to include source"
```

---

## Phase 1: Embedding

### Task 4: Embedder contract and deterministic fake

**Files:**
- Create: `sidecar/src/omni_memory/embedding/__init__.py`, `base.py`, `fake.py`
- Test: `sidecar/tests/test_fake_embedder.py`

**Interfaces:**
- Produces (`omni_memory.embedding.base.Embedder`, a `Protocol`): `model_id: str` (property), `dim: int` (property), `embed_documents(texts: Sequence[str]) -> np.ndarray` shape `(len(texts), dim)` float32 unit rows, `embed_images(images: Sequence[PIL.Image.Image]) -> np.ndarray` same shape contract, `embed_query(text: str) -> np.ndarray` shape `(dim,)` float32 unit.
- Produces (`omni_memory.embedding.fake.FakeEmbedder(dim: int = 256)`): satisfies `Embedder`; `model_id == "fake-hashed-bow-v1"`.

- [ ] **Step 1: Write the failing tests**

`sidecar/tests/test_fake_embedder.py`:

```python
import numpy as np
from PIL import Image

from omni_memory.embedding import FakeEmbedder


def test_rows_are_float32_unit_vectors():
    vecs = FakeEmbedder().embed_documents(["alpha beta", "gamma"])
    assert vecs.shape == (2, 256)
    assert vecs.dtype == np.float32
    np.testing.assert_allclose(np.linalg.norm(vecs, axis=1), 1.0, atol=1e-5)


def test_same_text_same_vector_across_instances():
    a = FakeEmbedder().embed_documents(["the quick fox"])
    b = FakeEmbedder().embed_documents(["the quick fox"])
    np.testing.assert_array_equal(a, b)


def test_shared_words_mean_higher_similarity():
    fake = FakeEmbedder()
    query = fake.embed_query("goroutine channels")
    related, unrelated = fake.embed_documents(
        ["channels connect goroutine workers", "recipe for chocolate cake"]
    )
    assert float(query @ related) > float(query @ unrelated) + 0.3


def test_query_has_shape_dim():
    assert FakeEmbedder(dim=64).embed_query("x").shape == (64,)


def test_empty_input_list_gives_empty_matrix():
    assert FakeEmbedder().embed_documents([]).shape == (0, 256)
    assert FakeEmbedder().embed_images([]).shape == (0, 256)


def test_empty_string_is_still_a_unit_vector():
    vec = FakeEmbedder().embed_query("")
    assert np.isfinite(vec).all()
    assert abs(float(np.linalg.norm(vec)) - 1.0) < 1e-5


def test_images_are_embedded_by_content():
    fake = FakeEmbedder()
    red = Image.new("RGB", (8, 8), "red")
    blue = Image.new("RGB", (8, 8), "blue")
    a, b, c = fake.embed_images([red, Image.new("RGB", (8, 8), "red"), blue])
    np.testing.assert_array_equal(a, b)
    assert float(a @ c) < 0.5
```

- [ ] **Step 2: Run to see it fail**

Run: `uv run pytest tests/test_fake_embedder.py -v`
Expected: FAIL (`ModuleNotFoundError: omni_memory.embedding`).

- [ ] **Step 3: Implement `embedding/base.py`**

```python
"""The Embedder contract: how text and images become unit vectors."""

from __future__ import annotations

from collections.abc import Sequence
from typing import TYPE_CHECKING, Protocol

import numpy as np

if TYPE_CHECKING:
    from PIL.Image import Image


class Embedder(Protocol):
    """Turns content into float32, L2-normalised vectors of length `dim`.

    Documents and queries are separate methods because the real model embeds them with
    different prompts (asymmetric retrieval).
    """

    @property
    def model_id(self) -> str: ...

    @property
    def dim(self) -> int: ...

    def embed_documents(self, texts: Sequence[str]) -> np.ndarray:
        """Shape `(len(texts), dim)`."""
        ...

    def embed_images(self, images: Sequence[Image]) -> np.ndarray:
        """Shape `(len(images), dim)`."""
        ...

    def embed_query(self, text: str) -> np.ndarray:
        """Shape `(dim,)`."""
        ...
```

- [ ] **Step 4: Implement `embedding/fake.py`**

```python
"""Deterministic stand-in for the real model, for tests that must not need a GPU.

Text becomes a hashed bag of words: every token owns a fixed pseudo-random direction and a
text is the normalised sum of its tokens' directions. Texts that share words therefore have
high cosine similarity, which is enough structure to exercise search code, and the same text
always yields the same vector. It is not semantic: synonyms are unrelated.
"""

from __future__ import annotations

import hashlib
import re
from collections.abc import Sequence
from functools import cache

import numpy as np
from PIL.Image import Image

_TOKEN = re.compile(r"[a-z0-9]+")
_EMPTY = "<empty>"


@cache
def _direction(token: str, dim: int) -> np.ndarray:
    seed = int.from_bytes(hashlib.sha256(token.encode()).digest()[:8], "little")
    vec = np.random.default_rng(seed).standard_normal(dim).astype(np.float32)
    vec /= np.linalg.norm(vec)
    vec.setflags(write=False)  # cached and shared: callers must not mutate it
    return vec


class FakeEmbedder:
    """Implements `Embedder` with hashed bags of words (text) and content hashes (images)."""

    def __init__(self, dim: int = 256) -> None:
        self._dim = dim

    @property
    def model_id(self) -> str:
        return "fake-hashed-bow-v1"

    @property
    def dim(self) -> int:
        return self._dim

    def _embed_tokens(self, tokens: list[str]) -> np.ndarray:
        if not tokens:
            return _direction(_EMPTY, self._dim).copy()
        total = np.sum([_direction(t, self._dim) for t in tokens], axis=0)
        norm = np.linalg.norm(total)
        if norm == 0:  # tokens cancelled out exactly; vanishingly unlikely but must stay unit
            return _direction(_EMPTY, self._dim).copy()
        return (total / norm).astype(np.float32)

    def _embed_text(self, text: str) -> np.ndarray:
        return self._embed_tokens(_TOKEN.findall(text.lower()))

    def _stack(self, rows: list[np.ndarray]) -> np.ndarray:
        if not rows:
            return np.zeros((0, self._dim), dtype=np.float32)
        return np.stack(rows).astype(np.float32)

    def embed_documents(self, texts: Sequence[str]) -> np.ndarray:
        return self._stack([self._embed_text(t) for t in texts])

    def embed_images(self, images: Sequence[Image]) -> np.ndarray:
        rows = []
        for image in images:
            digest = hashlib.sha256(f"{image.mode}{image.size}".encode() + image.tobytes())
            rows.append(self._embed_tokens([f"img:{digest.hexdigest()}"]))
        return self._stack(rows)

    def embed_query(self, text: str) -> np.ndarray:
        return self._embed_text(text)
```

- [ ] **Step 5: Implement `embedding/__init__.py`**

```python
"""Embedding models: the `Embedder` contract and its implementations.

`GemmaEmbedder` is deliberately not re-exported here: importing it pulls in torch, which
tests and tools that only need the contract should not pay for. Import it from
`omni_memory.embedding.gemma`.
"""

from omni_memory.embedding.base import Embedder
from omni_memory.embedding.fake import FakeEmbedder

__all__ = ["Embedder", "FakeEmbedder"]
```

- [ ] **Step 6: Gate and commit**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run pyright && uv run pytest -v`
Expected: all pass (7 new tests).

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(embedding): Embedder contract and deterministic FakeEmbedder"
```

### Task 5: GemmaEmbedder (real model)

**Files:**
- Create: `sidecar/src/omni_memory/embedding/gemma.py`
- Test: `sidecar/tests/test_gemma_embedder.py` (marked `model`)
- Modify: `docs/ARCHITECTURE.md`, `docs/STATUS.md`

**Interfaces:**
- Consumes: `Settings` (`model_id`, `dim`, `max_seq_length`, `text_batch_size`, `image_batch_size`) from Task 3; `Embedder` contract from Task 4.
- Produces: `GemmaEmbedder(settings: Settings, *, device: str | None = None)` satisfying `Embedder`, plus property `device -> str`. Defaults: `"cuda"` + bfloat16 if CUDA is available, else `"cpu"` + float32. Output vectors are re-normalised in float32 so rows are unit length to 1e-5.

- [ ] **Step 1: Write the model tests**

`sidecar/tests/test_gemma_embedder.py`:

```python
import numpy as np
import pytest
import torch
from PIL import Image, ImageDraw

from omni_memory.config import Settings
from omni_memory.embedding.gemma import GemmaEmbedder

pytestmark = pytest.mark.model

DOCS = [
    "Go channels let goroutines communicate safely.",
    "My flight to Hyderabad leaves at 6am on Friday.",
    "Svelte compiles components into small vanilla JS.",
    "Remember to buy milk and eggs.",
]


@pytest.fixture(scope="module")
def settings(tmp_path_factory):
    return Settings(index_dir=tmp_path_factory.mktemp("idx"))


@pytest.fixture(scope="module")
def embedder(settings):
    return GemmaEmbedder(settings)


def test_vectors_are_float32_unit_rows(embedder):
    vecs = embedder.embed_documents(DOCS)
    assert vecs.shape == (4, 256)
    assert vecs.dtype == np.float32
    np.testing.assert_allclose(np.linalg.norm(vecs, axis=1), 1.0, atol=1e-5)


def test_related_text_ranks_first(embedder):
    query = embedder.embed_query("how do goroutines talk to each other")
    scores = embedder.embed_documents(DOCS) @ query
    assert int(np.argmax(scores)) == 0
    assert scores[0] > 0.75  # spike measured 0.84
    assert np.sort(scores)[-2] < 0.7  # spike measured <= 0.60 for the others


def test_text_finds_matching_image(embedder):
    image = Image.new("RGB", (512, 512), "white")
    ImageDraw.Draw(image).rectangle([100, 300, 400, 320], fill="red")
    (image_vec,) = embedder.embed_images([image])
    red = embedder.embed_query("a red line on a white background") @ image_vec
    cat = embedder.embed_query("a photo of a cat") @ image_vec
    assert red > cat + 0.1  # spike measured 0.775 vs 0.559


def test_same_input_gives_same_vector(embedder):
    a = embedder.embed_documents(DOCS[:2])
    b = embedder.embed_documents(DOCS[:2])
    np.testing.assert_allclose(a, b, atol=1e-3)


def test_empty_inputs(embedder):
    assert embedder.embed_documents([]).shape == (0, 256)
    assert embedder.embed_images([]).shape == (0, 256)


@pytest.mark.skipif(not torch.cuda.is_available(), reason="needs CUDA")
def test_peak_vram_within_budget(embedder):
    torch.cuda.reset_peak_memory_stats()
    long_texts = ["word " * 600] * 16  # truncated to 512 tokens
    embedder.embed_documents(long_texts)
    embedder.embed_images([Image.new("RGB", (512, 512), "gray")] * 4)
    assert torch.cuda.max_memory_allocated() / 2**30 <= 1.5


@pytest.mark.skipif(not torch.cuda.is_available(), reason="needs CUDA")
def test_cpu_and_gpu_vectors_agree(embedder, settings):
    cpu = GemmaEmbedder(settings, device="cpu")
    assert embedder.device == "cuda" and cpu.device == "cpu"
    cosine = np.sum(embedder.embed_documents(DOCS) * cpu.embed_documents(DOCS), axis=1)
    print(f"min GPU(bf16) vs CPU(fp32) cosine: {cosine.min():.4f}")  # recorded in docs/STATUS.md
    assert cosine.min() > 0.98
```

- [ ] **Step 2: Run to see it fail**

Run: `uv run pytest -m model tests/test_gemma_embedder.py -v`
Expected: FAIL at collection (`ModuleNotFoundError: omni_memory.embedding.gemma`).

- [ ] **Step 3: Implement `embedding/gemma.py`**

```python
"""EmbeddingGemma 2 through sentence-transformers.

On the GPU the model runs in bfloat16 (about 0.8 GiB resident and 1.3 GiB peak at the batch
sizes in `Settings` on the dev laptop); on the CPU it runs in float32. Output vectors are
truncated to `Settings.dim` (Matryoshka) and L2-normalised in float32.
"""

from __future__ import annotations

from collections.abc import Sequence
from typing import TYPE_CHECKING, Any

import numpy as np
import torch
from sentence_transformers import SentenceTransformer

from omni_memory.config import Settings

if TYPE_CHECKING:
    from PIL.Image import Image


class GemmaEmbedder:
    """Implements `Embedder` with EmbeddingGemma 2 (text + vision; audio is a later spec)."""

    def __init__(self, settings: Settings, *, device: str | None = None) -> None:
        self._settings = settings
        self._device = device or ("cuda" if torch.cuda.is_available() else "cpu")
        dtype = torch.bfloat16 if self._device == "cuda" else torch.float32
        self._model = SentenceTransformer(
            settings.model_id,
            device=self._device,
            model_kwargs={"torch_dtype": dtype},
            config_kwargs={"audio_config": None},
        )
        self._model.max_seq_length = settings.max_seq_length

    @property
    def model_id(self) -> str:
        return self._settings.model_id

    @property
    def dim(self) -> int:
        return self._settings.dim

    @property
    def device(self) -> str:
        return self._device

    def _encode(
        self, inputs: Sequence[Any], *, prompt_name: str | None, batch_size: int
    ) -> np.ndarray:
        if len(inputs) == 0:
            return np.zeros((0, self.dim), dtype=np.float32)
        raw = self._model.encode(
            list(inputs),
            prompt_name=prompt_name,
            batch_size=batch_size,
            truncate_dim=self.dim,
            normalize_embeddings=True,
            show_progress_bar=False,
        )
        vecs = np.asarray(raw, dtype=np.float32)
        # bfloat16 normalisation is only accurate to ~1e-2; the store requires unit rows.
        return vecs / np.linalg.norm(vecs, axis=1, keepdims=True)

    def embed_documents(self, texts: Sequence[str]) -> np.ndarray:
        return self._encode(
            texts, prompt_name="Document", batch_size=self._settings.text_batch_size
        )

    def embed_images(self, images: Sequence[Image]) -> np.ndarray:
        return self._encode(
            [{"image": image} for image in images],
            prompt_name=None,
            batch_size=self._settings.image_batch_size,
        )

    def embed_query(self, text: str) -> np.ndarray:
        return self._encode([text], prompt_name="SearchQuery", batch_size=1)[0]
```

- [ ] **Step 4: Run the model tests**

Run: `uv run pytest -m model tests/test_gemma_embedder.py -v -s`
Expected: 7 passed (the model loads in ~10-15 s from the local cache). Note the printed `min GPU(bf16) vs CPU(fp32) cosine` value. If a test fails on a threshold, do not loosen it: re-measure, show the numbers to the owner, and decide together. If `transformers` warns that `torch_dtype` is deprecated, switch the keyword to `dtype`.

- [ ] **Step 5: Run the default gate (model tests excluded)**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run pyright && uv run pytest -v`
Expected: all non-model tests pass; model tests are deselected. If pyright objects to a `sentence_transformers` call, fix the types at that call; do not add blanket ignores.

- [ ] **Step 6: Phase 1 docs gate**

`docs/ARCHITECTURE.md`: set `embedding/` to `built`. Add under "## Invariants": `5. **Embedder output** is float32, shape (n, 256), rows unit length (1e-5); GPU runs bf16, CPU float32 (measured min cosine between them: <value from step 4>).` `docs/STATUS.md`: phase 1 `done`.

- [ ] **Step 7: Commit**

```bash
git add sidecar/src sidecar/tests docs
git commit -m "feat(embedding): GemmaEmbedder on GPU with CPU fallback"
```

---

## Phase 2: Store

### Task 6: Identifier splitting for keyword search

**Files:**
- Create: `sidecar/src/omni_memory/store/__init__.py`, `sidecar/src/omni_memory/store/identifiers.py`
- Test: `sidecar/tests/test_identifiers.py`

**Interfaces:** Produces `split_identifiers(text: str) -> str`: space-separated, lowercase, de-duplicated words found inside identifiers that contain more than one word (camelCase, PascalCase, snake_case, acronyms); `""` when there are none.

- [ ] **Step 1: Write the failing test**

`sidecar/tests/test_identifiers.py`:

```python
import pytest

from omni_memory.store.identifiers import split_identifiers


@pytest.mark.parametrize(
    ("text", "expected"),
    [
        ("addCountries", "add countries"),
        ("add_countries", "add countries"),
        ("HTTPServer", "http server"),
        ("getHTTPResponse", "get http response"),
        ("version2Beta", "version2 beta"),
        ("fooBar bazQux", "foo bar baz qux"),
        ("addCountries addCountries", "add countries"),
        ("plain words only", ""),
        ("__init__", ""),
        ("", ""),
    ],
)
def test_split_identifiers(text, expected):
    assert split_identifiers(text) == expected
```

- [ ] **Step 2: Run to see it fail**

Run: `uv run pytest tests/test_identifiers.py -v`
Expected: FAIL (`ModuleNotFoundError: omni_memory.store`).

- [ ] **Step 3: Implement**

`sidecar/src/omni_memory/store/__init__.py`:

```python
"""Persistent index: chunk metadata (SQLite) and vectors (memory-mapped file)."""
```

`sidecar/src/omni_memory/store/identifiers.py`:

```python
"""Splits code identifiers into words so keyword search finds `addCountries` by 'countries'."""

from __future__ import annotations

import re

_IDENTIFIER = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")
# lower/digit -> Upper boundary, acronym -> Word boundary (HTTPServer), or underscores.
_BOUNDARY = re.compile(r"(?<=[a-z0-9])(?=[A-Z])|(?<=[A-Z])(?=[A-Z][a-z])|_+")


def split_identifiers(text: str) -> str:
    """Words inside multi-word identifiers, lowercase, unique, in order of first appearance."""
    words: dict[str, None] = {}
    for identifier in _IDENTIFIER.findall(text):
        parts = [part for part in _BOUNDARY.split(identifier) if part]
        if len(parts) < 2:
            continue
        for part in parts:
            words[part.lower()] = None
    return " ".join(words)
```

- [ ] **Step 4: Gate and commit**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run pyright && uv run pytest -v`
Expected: all pass.

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(store): split code identifiers for keyword indexing"
```

### Task 7: VectorFile (memory-mapped, growable)

**Files:**
- Create: `sidecar/src/omni_memory/store/vectors.py`
- Test: `sidecar/tests/test_vector_file.py`

**Interfaces:**
- Produces `VectorFileError(Exception)` and `VectorFile(path: Path, dim: int, *, initial_capacity: int = 1024)`:
  - properties `dim -> int`, `capacity -> int`
  - `ensure_capacity(rows: int) -> None` (doubles until `rows` fit)
  - `write_rows(rows: Sequence[int], vectors: np.ndarray) -> None` (`vectors` shape `(len(rows), dim)`; grows as needed)
  - `read(row: int) -> np.ndarray` (a copy)
  - `scores(query: np.ndarray, upto: int) -> np.ndarray` (float32 dot products of rows `[0, upto)` with `query`; a fresh array)
  - `flush() -> None` (flush mapping and fsync), `close() -> None` (idempotent, releases the file)
- Not thread-safe; `FlatStore` serialises access. No numpy view of the mapping ever leaves the class.

- [ ] **Step 1: Write the failing tests**

`sidecar/tests/test_vector_file.py`:

```python
import numpy as np
import pytest

from omni_memory.store.vectors import VectorFile, VectorFileError


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
```

- [ ] **Step 2: Run to see it fail**

Run: `uv run pytest tests/test_vector_file.py -v`
Expected: FAIL (`ModuleNotFoundError: omni_memory.store.vectors`).

- [ ] **Step 3: Implement `store/vectors.py`**

```python
"""A growable, memory-mapped matrix of float32 vectors stored in one file.

Layout: a 64-byte header (magic, version, dim, capacity) followed by `capacity` rows of
`dim` little-endian float32. Row r starts at byte 64 + r * dim * 4. The file grows by
doubling. Windows cannot resize a mapped file, so growth unmaps, truncates and remaps; that
is only safe because no numpy view of the mapping ever leaves this class (`scores` and
`read` return fresh arrays).

Not thread-safe: the owner (`FlatStore`) serialises access.
"""

from __future__ import annotations

import mmap
import os
import struct
from collections.abc import Sequence
from pathlib import Path

import numpy as np

_MAGIC = b"OMNIVEC1"
_VERSION = 1
_HEADER = struct.Struct("<8sIIQ")  # magic, version, dim, capacity
HEADER_SIZE = 64
_ITEM_SIZE = 4  # bytes per float32


class VectorFileError(Exception):
    """The file is not a valid vector file for the requested dimension."""


class VectorFile:
    """Rows of `dim` float32 values addressed by row number."""

    def __init__(self, path: Path, dim: int, *, initial_capacity: int = 1024) -> None:
        if initial_capacity < 1:
            raise ValueError(f"initial_capacity must be >= 1, got {initial_capacity}")
        self._path = path
        self._dim = dim
        existed = path.exists()
        # closed in close() or on a failed open below
        self._file = open(path, "r+b" if existed else "w+b")  # noqa: SIM115
        try:
            if existed:
                self._capacity = self._read_header()
            else:
                self._capacity = initial_capacity
                self._file.truncate(self._size_for(self._capacity))
                self._write_header()
            self._map()
        except BaseException:
            self._file.close()
            raise

    @property
    def dim(self) -> int:
        return self._dim

    @property
    def capacity(self) -> int:
        return self._capacity

    def _size_for(self, capacity: int) -> int:
        return HEADER_SIZE + capacity * self._dim * _ITEM_SIZE

    def _write_header(self) -> None:
        header = _HEADER.pack(_MAGIC, _VERSION, self._dim, self._capacity)
        self._file.seek(0)
        self._file.write(header.ljust(HEADER_SIZE, b"\0"))
        self._file.flush()

    def _read_header(self) -> int:
        self._file.seek(0)
        raw = self._file.read(_HEADER.size)
        if len(raw) < _HEADER.size:
            raise VectorFileError(f"{self._path}: truncated header")
        magic, version, dim, capacity = _HEADER.unpack(raw)
        if magic != _MAGIC or version != _VERSION:
            raise VectorFileError(f"{self._path}: not a vector file (magic/version mismatch)")
        if dim != self._dim:
            raise VectorFileError(f"{self._path}: holds dim {dim}, expected dim {self._dim}")
        if os.fstat(self._file.fileno()).st_size < self._size_for(capacity):
            raise VectorFileError(f"{self._path}: file is shorter than its header claims")
        return capacity

    def _map(self) -> None:
        self._mm = mmap.mmap(self._file.fileno(), self._size_for(self._capacity))
        flat = np.frombuffer(
            self._mm, dtype="<f4", offset=HEADER_SIZE, count=self._capacity * self._dim
        )
        self._matrix: np.ndarray | None = flat.reshape(self._capacity, self._dim)

    def _unmap(self) -> None:
        self._matrix = None  # drop every numpy view first, or mmap.close() raises BufferError
        self._mm.flush()
        self._mm.close()

    @property
    def _rows(self) -> np.ndarray:
        if self._matrix is None:
            raise ValueError("vector file is closed")
        return self._matrix

    def ensure_capacity(self, rows: int) -> None:
        """Grow (doubling) until at least `rows` rows fit."""
        if rows <= self._capacity:
            return
        capacity = self._capacity
        while capacity < rows:
            capacity *= 2
        self._unmap()
        self._file.truncate(self._size_for(capacity))
        self._capacity = capacity
        self._write_header()
        self._map()

    def write_rows(self, rows: Sequence[int], vectors: np.ndarray) -> None:
        """Store `vectors[i]` at row `rows[i]`, growing the file if needed."""
        expected = (len(rows), self._dim)
        if vectors.shape != expected:
            raise ValueError(f"vectors must have shape {expected}, got {vectors.shape}")
        if len(rows) == 0:
            return
        index = np.asarray(rows, dtype=np.intp)
        self.ensure_capacity(int(index.max()) + 1)
        self._rows[index] = vectors

    def read(self, row: int) -> np.ndarray:
        """A copy of one row."""
        return self._rows[row].copy()

    def scores(self, query: np.ndarray, upto: int) -> np.ndarray:
        """Dot products of `query` with rows `[0, upto)`: a fresh array, safe to keep."""
        return self._rows[:upto] @ query

    def flush(self) -> None:
        """Push written rows to disk."""
        self._mm.flush()
        os.fsync(self._file.fileno())

    def close(self) -> None:
        if self._file.closed:
            return
        self._unmap()
        self._file.close()
```

- [ ] **Step 4: Gate and commit**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run pyright && uv run pytest -v`
Expected: all pass (10 new tests). ruff format will re-wrap the long `ValueError` line.

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(store): memory-mapped growable VectorFile"
```

### Task 8: FlatStore write path (schema, upsert, delete, get, reopen)

**Files:**
- Create: `sidecar/src/omni_memory/store/schema.py`, `sidecar/src/omni_memory/store/flat.py`, `sidecar/tests/helpers.py`, `sidecar/tests/conftest.py`
- Modify: `sidecar/src/omni_memory/store/__init__.py`, `docs/superpowers/specs/2026-10-12-memory-engine-design.md` (section 5)
- Test: `sidecar/tests/test_flat_store.py`

**Interfaces:**
- Consumes: `VectorFile` (Task 7), `split_identifiers` (Task 6), `Chunk`, `ChunkKind`, `Filters`, `VectorHit` (Task 3).
- Produces `omni_memory.store.FlatStore(directory: Path, *, model_id: str, dim: int)` with:
  - `upsert(chunks: Sequence[Chunk], vectors: np.ndarray) -> None` (vectors float32 `(len, dim)`, unit rows, finite; duplicate ids in the batch raise `ValueError`)
  - `delete(ids: Iterable[str]) -> int` (number that existed)
  - `get(ids: Sequence[str]) -> dict[str, Chunk]` (missing ids absent)
  - `count() -> int`, `close() -> None`
  - `IndexMismatchError` raised when reopened with a different `model_id`, `dim` or schema version.
- Produces test helpers: `tests.helpers.DIM = 8`, `make_chunk(name, text="", *, source_id="s", kind=ChunkKind.TEXT, mtime_ns=1) -> Chunk`, `unit_vectors(n, seed=0) -> np.ndarray`; fixture `store` (new `FlatStore` in a temp dir, closed afterwards).

- [ ] **Step 1: Test helpers**

`sidecar/tests/helpers.py`:

```python
import numpy as np

from omni_memory.types import Chunk, ChunkKind

DIM = 8


def make_chunk(
    name: str,
    text: str = "",
    *,
    source_id: str = "s",
    kind: ChunkKind = ChunkKind.TEXT,
    mtime_ns: int = 1,
) -> Chunk:
    return Chunk(
        source_id=source_id,
        uri=f"{name}.txt",
        kind=kind,
        locator='{"lines":[1,1]}',
        content_hash=name,
        mtime_ns=mtime_ns,
        text=text or name,
    )


def unit_vectors(n: int, seed: int = 0, dim: int = DIM) -> np.ndarray:
    raw = np.random.default_rng(seed).standard_normal((n, dim)).astype(np.float32)
    return raw / np.linalg.norm(raw, axis=1, keepdims=True)
```

`sidecar/tests/conftest.py`:

```python
import pytest
from helpers import DIM

from omni_memory.store import FlatStore


@pytest.fixture
def store(tmp_path):
    flat = FlatStore(tmp_path / "idx", model_id="fake", dim=DIM)
    yield flat
    flat.close()
```

- [ ] **Step 2: Write the failing tests**

`sidecar/tests/test_flat_store.py`:

```python
import shutil
import sqlite3

import numpy as np
import pytest
from helpers import DIM, make_chunk, unit_vectors

from omni_memory.store import FlatStore, IndexMismatchError


def fts_ids(directory, term):
    """Ids the keyword index finds for `term`, read through a separate connection."""
    db = sqlite3.connect(directory / "meta.db")
    try:
        sql = (
            "SELECT c.id FROM chunks_fts f JOIN chunks c ON c.rid = f.rowid "
            "WHERE chunks_fts MATCH ?"
        )
        return {row[0] for row in db.execute(sql, (term,))}
    finally:
        db.close()


def test_upsert_then_get_roundtrip(store):
    a, b = make_chunk("a"), make_chunk("b", "other text")
    store.upsert([a, b], unit_vectors(2))
    assert store.get([a.id, b.id]) == {a.id: a, b.id: b}
    assert store.count() == 2


def test_new_store_is_empty(store):
    assert store.count() == 0
    assert store.get(["nope"]) == {}


def test_upserting_an_existing_id_updates_in_place(store):
    store.upsert([make_chunk("a", mtime_ns=1)], unit_vectors(1))
    newer = make_chunk("a", mtime_ns=2)
    store.upsert([newer], unit_vectors(1, seed=1))
    assert store.count() == 1
    assert store.get([newer.id])[newer.id].mtime_ns == 2


def test_duplicate_ids_in_one_batch_are_rejected(store):
    with pytest.raises(ValueError, match="duplicate"):
        store.upsert([make_chunk("a"), make_chunk("a")], unit_vectors(2))
    assert store.count() == 0


def test_empty_batch_is_a_noop(store):
    store.upsert([], np.zeros((0, DIM), dtype=np.float32))
    assert store.count() == 0


def bad_vectors():
    good = unit_vectors(1)
    nan = good.copy()
    nan[0, 0] = np.nan
    return {
        "wrong dimension": unit_vectors(1, dim=DIM + 1),
        "float64": good.astype(np.float64),
        "nan": nan,
        "not unit length": good * 2,
        "wrong row count": unit_vectors(2),
    }


@pytest.mark.parametrize("label", list(bad_vectors()))
def test_bad_vectors_are_rejected_before_touching_disk(store, label):
    with pytest.raises(ValueError):
        store.upsert([make_chunk("a")], bad_vectors()[label])
    assert store.count() == 0


def test_delete_removes_and_reports_how_many_existed(store):
    a, b = make_chunk("a"), make_chunk("b")
    store.upsert([a, b], unit_vectors(2))
    assert store.delete([a.id, "missing"]) == 1
    assert store.get([a.id, b.id]) == {b.id: b}
    assert store.count() == 1


def test_everything_survives_reopen(tmp_path):
    directory = tmp_path / "idx"
    first = FlatStore(directory, model_id="fake", dim=DIM)
    chunks = [make_chunk("a"), make_chunk("b")]
    first.upsert(chunks, unit_vectors(2))
    first.close()

    second = FlatStore(directory, model_id="fake", dim=DIM)
    assert second.count() == 2
    assert set(second.get([c.id for c in chunks])) == {c.id for c in chunks}
    second.close()


def test_reopening_with_another_model_or_dimension_is_refused(tmp_path):
    directory = tmp_path / "idx"
    FlatStore(directory, model_id="m1", dim=DIM).close()
    with pytest.raises(IndexMismatchError, match="reindex"):
        FlatStore(directory, model_id="m2", dim=DIM)
    with pytest.raises(IndexMismatchError, match="reindex"):
        FlatStore(directory, model_id="m1", dim=DIM + 1)


def test_keyword_index_follows_writes(tmp_path):
    directory = tmp_path / "idx"
    store = FlatStore(directory, model_id="fake", dim=DIM)
    original = make_chunk("a", "addCountries zebra")
    store.upsert([original], unit_vectors(1))
    assert fts_ids(directory, "zebra") == {original.id}
    assert fts_ids(directory, "countries") == {original.id}  # via split identifiers
    assert fts_ids(directory, "addcountries") == {original.id}  # via the original token

    replaced = make_chunk("a", "now says giraffe", mtime_ns=2)
    store.upsert([replaced], unit_vectors(1))
    assert fts_ids(directory, "zebra") == set()
    assert fts_ids(directory, "giraffe") == {replaced.id}

    store.delete([replaced.id])
    assert fts_ids(directory, "giraffe") == set()
    store.close()


def test_hostile_text_is_stored_verbatim(store):
    nasty = 'say "hi" * NEAR( x ) \'quoted\' 🙂 line1\r\nline2'
    chunk = make_chunk("a", nasty)
    store.upsert([chunk], unit_vectors(1))
    assert store.get([chunk.id])[chunk.id].text == nasty


def test_closed_store_leaves_no_open_handles(tmp_path):
    directory = tmp_path / "idx"
    store = FlatStore(directory, model_id="fake", dim=DIM)
    store.upsert([make_chunk("a")], unit_vectors(1))
    store.close()
    shutil.rmtree(directory)  # raises PermissionError on Windows if a handle leaked
```

- [ ] **Step 3: Run to see them fail**

Run: `uv run pytest tests/test_flat_store.py -v`
Expected: FAIL at collection (`ImportError: cannot import name 'FlatStore' from 'omni_memory.store'`).

- [ ] **Step 4: Implement `store/schema.py`**

```python
"""SQL for meta.db, schema version 1: index identity, chunk metadata and the keyword index.

`chunks.rid` is an explicit integer primary key (not the implicit rowid) so the external-content
FTS table keeps pointing at the right rows even if the database is ever VACUUMed.
"""

SCHEMA_VERSION = 1

SCHEMA = """
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS chunks (
    rid          INTEGER PRIMARY KEY,
    id           TEXT NOT NULL UNIQUE,
    vec_row      INTEGER NOT NULL UNIQUE,
    source_id    TEXT NOT NULL,
    uri          TEXT NOT NULL,
    kind         TEXT NOT NULL,
    locator      TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    mtime_ns     INTEGER NOT NULL,
    text         TEXT NOT NULL,
    identifiers  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS chunks_source ON chunks(source_id);

CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
    text, identifiers,
    content='chunks', content_rowid='rid',
    tokenize='unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
    INSERT INTO chunks_fts(rowid, text, identifiers)
    VALUES (new.rid, new.text, new.identifiers);
END;
CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, text, identifiers)
    VALUES ('delete', old.rid, old.text, old.identifiers);
END;
CREATE TRIGGER IF NOT EXISTS chunks_au AFTER UPDATE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, text, identifiers)
    VALUES ('delete', old.rid, old.text, old.identifiers);
    INSERT INTO chunks_fts(rowid, text, identifiers)
    VALUES (new.rid, new.text, new.identifiers);
END;
"""
```

- [ ] **Step 5: Implement `store/flat.py` (write path; search arrives in Task 9)**

```python
"""FlatStore: chunk metadata plus exact vector search, in one directory.

Directory layout: `meta.db` (SQLite, WAL) and `vectors.f32` (see `VectorFile`).

Invariants (mirrored in docs/ARCHITECTURE.md):
1. SQLite is the source of truth: a chunk exists iff it has a row in `chunks`. Vector rows
   without a `chunks` entry are garbage and may be overwritten.
2. Vectors are flushed to disk before the SQLite transaction that references them commits,
   so a committed chunk always points at a durable vector.
3. Every public method holds `self._lock`; the vector file is remapped when it grows, so no
   reader may overlap a writer.
4. Ranked output is ordered by (-score, id), so ties never depend on row placement.
"""

from __future__ import annotations

import heapq
import sqlite3
import threading
from collections.abc import Iterable, Iterator, Sequence
from pathlib import Path

import numpy as np

from omni_memory.store.identifiers import split_identifiers
from omni_memory.store.schema import SCHEMA, SCHEMA_VERSION
from omni_memory.store.vectors import VectorFile
from omni_memory.types import Chunk, ChunkKind

_UNIT_TOLERANCE = 1e-3
_SQL_BATCH = 500  # far below SQLite's bound-variable limit

_UPSERT = """
INSERT INTO chunks (id, vec_row, source_id, uri, kind, locator, content_hash, mtime_ns,
                    text, identifiers)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    source_id = excluded.source_id, uri = excluded.uri, kind = excluded.kind,
    locator = excluded.locator, content_hash = excluded.content_hash,
    mtime_ns = excluded.mtime_ns, text = excluded.text, identifiers = excluded.identifiers
"""


class IndexMismatchError(Exception):
    """The index on disk was built with a different model, dimension or schema."""


def _batches[T](items: Sequence[T], size: int = _SQL_BATCH) -> Iterator[Sequence[T]]:
    for start in range(0, len(items), size):
        yield items[start : start + size]


class FlatStore:
    """Exact-search store. One instance owns its directory."""

    def __init__(self, directory: Path, *, model_id: str, dim: int) -> None:
        directory.mkdir(parents=True, exist_ok=True)
        self._dim = dim
        self._lock = threading.RLock()
        self._db = sqlite3.connect(directory / "meta.db", check_same_thread=False)
        try:
            self._init_db(model_id, dim)
            self._vectors = VectorFile(directory / "vectors.f32", dim)
        except BaseException:
            self._db.close()
            raise
        self._load_rows()

    def _init_db(self, model_id: str, dim: int) -> None:
        self._db.execute("PRAGMA journal_mode=WAL")
        self._db.execute("PRAGMA synchronous=FULL")
        self._db.executescript(SCHEMA)
        stored = dict(self._db.execute("SELECT key, value FROM meta").fetchall())
        if not stored:
            with self._db:
                self._db.executemany(
                    "INSERT INTO meta VALUES (?, ?)",
                    [
                        ("schema_version", str(SCHEMA_VERSION)),
                        ("model_id", model_id),
                        ("dim", str(dim)),
                    ],
                )
        elif (
            stored.get("model_id") != model_id
            or stored.get("dim") != str(dim)
            or stored.get("schema_version") != str(SCHEMA_VERSION)
        ):
            built = (stored.get("model_id"), stored.get("dim"), stored.get("schema_version"))
            wanted = (model_id, str(dim), str(SCHEMA_VERSION))
            raise IndexMismatchError(
                f"index was built with (model, dim, schema) = {built}; "
                f"settings want {wanted}: reindex required"
            )

    def _load_rows(self) -> None:
        used = [row for (row,) in self._db.execute("SELECT vec_row FROM chunks")]
        self._high_water = max(used, default=-1) + 1  # rows [0, high_water) were ever allocated
        taken = set(used)
        self._free = [row for row in range(self._high_water) if row not in taken]
        heapq.heapify(self._free)  # already ascending, so already a valid heap
        self._live = np.zeros(self._vectors.capacity, dtype=bool)
        self._live[np.asarray(used, dtype=np.intp)] = True

    # --- validation -------------------------------------------------------------------

    def _validate_batch(self, chunks: Sequence[Chunk], vectors: np.ndarray) -> None:
        expected = (len(chunks), self._dim)
        if vectors.dtype != np.float32 or vectors.shape != expected:
            raise ValueError(
                f"vectors must be float32 with shape {expected}, "
                f"got {vectors.dtype} {vectors.shape}"
            )
        if len(chunks) == 0:
            return
        if not np.isfinite(vectors).all():
            raise ValueError("vectors contain NaN or infinity")
        if np.abs(np.linalg.norm(vectors, axis=1) - 1.0).max() > _UNIT_TOLERANCE:
            raise ValueError("vectors must be L2-normalised")
        ids = [chunk.id for chunk in chunks]
        if len(set(ids)) != len(ids):
            raise ValueError("duplicate chunk ids in one batch")

    # --- row bookkeeping --------------------------------------------------------------

    def _allocate_row(self) -> int:
        if self._free:
            return heapq.heappop(self._free)  # lowest free row first: deterministic reuse
        row = self._high_water
        self._high_water += 1
        return row

    def _sync_live_size(self) -> None:
        short = self._vectors.capacity - self._live.size
        if short > 0:
            self._live = np.concatenate([self._live, np.zeros(short, dtype=bool)])

    def _rows_for(self, ids: Sequence[str]) -> dict[str, int]:
        found: dict[str, int] = {}
        for batch in _batches(ids):
            marks = ",".join("?" * len(batch))
            query = f"SELECT id, vec_row FROM chunks WHERE id IN ({marks})"
            found.update(self._db.execute(query, list(batch)).fetchall())
        return found

    # --- public API -------------------------------------------------------------------

    def upsert(self, chunks: Sequence[Chunk], vectors: np.ndarray) -> None:
        """Insert or replace chunks and their vectors as one durable batch."""
        self._validate_batch(chunks, vectors)
        if len(chunks) == 0:
            return
        with self._lock:
            existing = self._rows_for([chunk.id for chunk in chunks])
            rows: list[int] = []
            fresh: list[int] = []
            for chunk in chunks:
                row = existing.get(chunk.id)
                if row is None:
                    row = self._allocate_row()
                    fresh.append(row)
                rows.append(row)
            try:
                self._vectors.write_rows(rows, vectors)
                self._sync_live_size()
                self._vectors.flush()  # invariant 2: durable before metadata commits
                self._write_metadata(chunks, rows)
            except BaseException:
                for row in fresh:
                    heapq.heappush(self._free, row)
                raise
            self._live[np.asarray(rows, dtype=np.intp)] = True

    def _write_metadata(self, chunks: Sequence[Chunk], rows: Sequence[int]) -> None:
        records = [
            (
                chunk.id,
                row,
                chunk.source_id,
                chunk.uri,
                chunk.kind.value,
                chunk.locator,
                chunk.content_hash,
                chunk.mtime_ns,
                chunk.text,
                split_identifiers(chunk.text),
            )
            for chunk, row in zip(chunks, rows, strict=True)
        ]
        with self._db:
            self._db.executemany(_UPSERT, records)

    def delete(self, ids: Iterable[str]) -> int:
        """Remove chunks by id; returns how many existed."""
        with self._lock:
            rows = self._rows_for(list(ids))
            if not rows:
                return 0
            with self._db:
                self._db.executemany("DELETE FROM chunks WHERE id = ?", [(i,) for i in rows])
            self._live[np.asarray(list(rows.values()), dtype=np.intp)] = False
            for row in rows.values():
                heapq.heappush(self._free, row)
            return len(rows)

    def get(self, ids: Sequence[str]) -> dict[str, Chunk]:
        """Chunk metadata by id; ids that do not exist are absent from the result."""
        found: dict[str, Chunk] = {}
        with self._lock:
            for batch in _batches(ids):
                marks = ",".join("?" * len(batch))
                query = (
                    "SELECT id, source_id, uri, kind, locator, content_hash, mtime_ns, text "
                    f"FROM chunks WHERE id IN ({marks})"
                )
                for id_, source_id, uri, kind, locator, content_hash, mtime_ns, text in (
                    self._db.execute(query, list(batch))
                ):
                    found[id_] = Chunk(
                        source_id=source_id,
                        uri=uri,
                        kind=ChunkKind(kind),
                        locator=locator,
                        content_hash=content_hash,
                        mtime_ns=mtime_ns,
                        text=text,
                    )
        return found

    def count(self) -> int:
        with self._lock:
            (total,) = self._db.execute("SELECT COUNT(*) FROM chunks").fetchone()
            return total

    def close(self) -> None:
        """Release the database and the vector file (both are removable afterwards)."""
        with self._lock:
            self._vectors.close()
            self._db.close()
```

- [ ] **Step 6: Export from the package**

Replace `sidecar/src/omni_memory/store/__init__.py` with:

```python
"""Persistent index: chunk metadata (SQLite) and vectors (memory-mapped file)."""

from omni_memory.store.flat import FlatStore, IndexMismatchError

__all__ = ["FlatStore", "IndexMismatchError"]
```

- [ ] **Step 7: Run the tests, then fix the spec**

Run: `uv run pytest tests/test_flat_store.py -v`
Expected: 16 passed (the bad-vectors test is parametrised 5 ways).

In `docs/superpowers/specs/2026-10-12-memory-engine-design.md` section 5 replace ``` `chunks(id TEXT PRIMARY KEY, row INTEGER UNIQUE, kind TEXT, source_id TEXT, uri TEXT, locator TEXT, content_hash TEXT, mtime_ns INTEGER, text TEXT)` ``` with ``` `chunks(rid INTEGER PRIMARY KEY, id TEXT UNIQUE, vec_row INTEGER UNIQUE, kind TEXT, source_id TEXT, uri TEXT, locator TEXT, content_hash TEXT, mtime_ns INTEGER, text TEXT, identifiers TEXT)` ``` and replace "`row` is the chunk's row in the vector file." with "`vec_row` is the chunk's row in the vector file (`row` is a SQLite keyword)."

- [ ] **Step 8: Gate and commit**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run pyright && uv run pytest -v`
Expected: all pass.

```bash
git add sidecar/src sidecar/tests docs
git commit -m "feat(store): FlatStore write path with FTS-synced metadata"
```

### Task 9: FlatStore exact search

**Files:**
- Modify: `sidecar/src/omni_memory/store/flat.py`
- Test: `sidecar/tests/test_flat_search.py`

**Interfaces:**
- Consumes: Task 8's `FlatStore`, `Filters`, `VectorHit`.
- Produces: `FlatStore.search_vectors(query: np.ndarray, n: int, filters: Filters | None = None) -> list[VectorHit]`: exact top-`n` by dot product (cosine for unit queries) among live chunks that match `filters`, ordered by `(-score, id)`; `[]` when nothing qualifies; `ValueError` when `query` is not finite or not shape `(dim,)`.

- [ ] **Step 1: Write the failing tests**

`sidecar/tests/test_flat_search.py`:

```python
import threading

import numpy as np
import pytest
from helpers import DIM, make_chunk, unit_vectors

from omni_memory.store import FlatStore
from omni_memory.types import ChunkKind, Filters


def oracle(vectors_by_id: dict[str, np.ndarray], query: np.ndarray, n: int):
    """Plain brute force in float64: the reference every store result is compared to."""
    q = query.astype(np.float64)
    scored = [(float(v.astype(np.float64) @ q), id_) for id_, v in vectors_by_id.items()]
    return sorted(scored, key=lambda item: (-item[0], item[1]))[:n]


def fill(store, count: int, seed: int = 0, **chunk_args):
    chunks = [make_chunk(f"c{i}", **chunk_args) for i in range(count)]
    vectors = unit_vectors(count, seed)
    store.upsert(chunks, vectors)
    return chunks, vectors


def test_matches_the_brute_force_oracle(store):
    chunks, vectors = fill(store, 500)
    by_id = {c.id: v for c, v in zip(chunks, vectors, strict=True)}
    for query in unit_vectors(20, seed=99):
        hits = store.search_vectors(query, 10)
        expected = oracle(by_id, query, 10)
        assert [h.id for h in hits] == [id_ for _, id_ in expected]
        np.testing.assert_allclose([h.score for h in hits], [s for s, _ in expected], atol=1e-5)


def test_empty_store_returns_nothing(store):
    assert store.search_vectors(unit_vectors(1)[0], 10) == []


def test_asking_for_more_than_exists_returns_everything(store):
    chunks, _ = fill(store, 3)
    hits = store.search_vectors(unit_vectors(1, seed=5)[0], 100)
    assert {h.id for h in hits} == {c.id for c in chunks}


def test_non_positive_n_returns_nothing(store):
    fill(store, 3)
    assert store.search_vectors(unit_vectors(1)[0], 0) == []


def test_deleted_chunks_are_not_returned(store):
    chunks, vectors = fill(store, 5)
    store.delete([chunks[0].id])
    hits = store.search_vectors(vectors[0], 10)
    assert chunks[0].id not in {h.id for h in hits}
    assert len(hits) == 4


def test_a_reinserted_id_uses_its_latest_vector(store):
    chunk = make_chunk("a")
    old, new = unit_vectors(2, seed=1)
    store.upsert([chunk], old[None, :])
    store.upsert([chunk], new[None, :])
    (hit,) = store.search_vectors(new, 1)
    assert hit.id == chunk.id
    assert hit.score == pytest.approx(1.0, abs=1e-5)


def test_freed_rows_are_reused_without_resurrecting_old_vectors(store):
    chunks, vectors = fill(store, 3)
    store.delete([chunks[1].id])
    newcomer = make_chunk("new")
    fresh = unit_vectors(1, seed=7)
    store.upsert([newcomer], fresh)
    hits = store.search_vectors(vectors[1], 10)  # the deleted chunk's old vector as the query
    assert chunks[1].id not in {h.id for h in hits}
    assert {h.id for h in hits} == {chunks[0].id, chunks[2].id, newcomer.id}


def test_results_survive_reopen(tmp_path):
    directory = tmp_path / "idx"
    first = FlatStore(directory, model_id="fake", dim=DIM)
    chunks, vectors = fill(first, 50)
    before = first.search_vectors(vectors[3], 5)
    first.close()

    second = FlatStore(directory, model_id="fake", dim=DIM)
    assert second.search_vectors(vectors[3], 5) == before
    second.close()


def test_ties_are_broken_by_id_whatever_the_insert_order(store):
    chunks = [make_chunk(name) for name in ("zulu", "alpha", "mike")]
    same = np.repeat(unit_vectors(1), 3, axis=0)
    store.upsert(chunks, same)
    hits = store.search_vectors(same[0], 3)
    assert [h.id for h in hits] == sorted(c.id for c in chunks)
    assert [h.id for h in store.search_vectors(same[0], 2)] == sorted(c.id for c in chunks)[:2]


def test_filters_restrict_candidates(store):
    code = make_chunk("a", kind=ChunkKind.CODE, source_id="repo", mtime_ns=10)
    doc = make_chunk("b", kind=ChunkKind.PDF_TEXT, source_id="docs", mtime_ns=20)
    img = make_chunk("c", kind=ChunkKind.IMAGE, source_id="docs", mtime_ns=30)
    store.upsert([code, doc, img], unit_vectors(3))
    query = unit_vectors(1, seed=2)[0]

    def ids(filters):
        return {h.id for h in store.search_vectors(query, 10, filters)}

    assert ids(None) == {code.id, doc.id, img.id}
    assert ids(Filters(kinds=frozenset({ChunkKind.CODE}))) == {code.id}
    assert ids(Filters(kinds=frozenset())) == set()
    assert ids(Filters(source_id="docs")) == {doc.id, img.id}
    assert ids(Filters(uri_prefix="b")) == {doc.id}
    assert ids(Filters(modified_after_ns=15)) == {doc.id, img.id}
    assert ids(Filters(source_id="docs", modified_after_ns=25)) == {img.id}


@pytest.mark.parametrize("bad", [np.ones(DIM + 1, np.float32), np.full(DIM, np.nan, np.float32)])
def test_bad_queries_are_rejected(store, bad):
    fill(store, 2)
    with pytest.raises(ValueError):
        store.search_vectors(bad, 5)


def test_searching_while_the_vector_file_grows_does_not_crash(store):
    fill(store, 10)
    failures: list[BaseException] = []
    stop = threading.Event()

    def searcher():
        query = unit_vectors(1, seed=3)[0]
        try:
            while not stop.is_set():
                store.search_vectors(query, 5)
        except BaseException as error:
            failures.append(error)

    thread = threading.Thread(target=searcher)
    thread.start()
    try:
        for batch in range(6):  # 6 x 500 chunks pushes past the initial 1024 rows: two growths
            chunks = [make_chunk(f"g{batch}-{i}") for i in range(500)]
            store.upsert(chunks, unit_vectors(500, seed=batch + 10))
    finally:
        stop.set()
        thread.join()
    assert failures == []
    assert store.count() == 3010
```

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_flat_search.py -v`
Expected: FAIL (`AttributeError: 'FlatStore' object has no attribute 'search_vectors'`).

- [ ] **Step 3: Implement search**

In `sidecar/src/omni_memory/store/flat.py` change the import line `from omni_memory.types import Chunk, ChunkKind` to:

```python
from omni_memory.types import Chunk, ChunkKind, Filters, VectorHit
```

Add these methods to `FlatStore`, after `count()` and before `close()`:

```python
    def _validate_query(self, query: np.ndarray) -> np.ndarray:
        vec = np.asarray(query, dtype=np.float32)
        if vec.shape != (self._dim,):
            raise ValueError(f"query must have shape ({self._dim},), got {vec.shape}")
        if not np.isfinite(vec).all():
            raise ValueError("query contains NaN or infinity")
        return vec

    def _filter_mask(self, filters: Filters, size: int) -> np.ndarray:
        """Boolean mask over rows [0, size) for chunks that satisfy `filters`."""
        clauses: list[str] = []
        params: list[object] = []
        if filters.kinds is not None:
            if not filters.kinds:
                return np.zeros(size, dtype=bool)
            clauses.append(f"kind IN ({','.join('?' * len(filters.kinds))})")
            params.extend(sorted(kind.value for kind in filters.kinds))
        if filters.source_id is not None:
            clauses.append("source_id = ?")
            params.append(filters.source_id)
        if filters.uri_prefix is not None:
            clauses.append("substr(uri, 1, ?) = ?")
            params.extend([len(filters.uri_prefix), filters.uri_prefix])
        if filters.modified_after_ns is not None:
            clauses.append("mtime_ns > ?")
            params.append(filters.modified_after_ns)
        if not clauses:
            return np.ones(size, dtype=bool)
        query = f"SELECT vec_row FROM chunks WHERE {' AND '.join(clauses)}"
        mask = np.zeros(size, dtype=bool)
        rows = [row for (row,) in self._db.execute(query, params)]
        mask[np.asarray(rows, dtype=np.intp)] = True
        return mask

    def _ids_for_rows(self, rows: Sequence[int]) -> dict[int, str]:
        found: dict[int, str] = {}
        for batch in _batches(rows):
            marks = ",".join("?" * len(batch))
            query = f"SELECT vec_row, id FROM chunks WHERE vec_row IN ({marks})"
            found.update(self._db.execute(query, list(batch)).fetchall())
        return found

    def search_vectors(
        self, query: np.ndarray, n: int, filters: Filters | None = None
    ) -> list[VectorHit]:
        """Exact top-`n` by dot product among live chunks matching `filters`.

        Ordered by (-score, id). Candidates tied with the n-th score are all kept before the
        final sort, so which of several equal scores make the cut never depends on row order.
        """
        vec = self._validate_query(query)
        with self._lock:
            size = self._high_water
            if size == 0 or n <= 0:
                return []
            allowed = self._live[:size].copy()
            if filters is not None:
                allowed &= self._filter_mask(filters, size)
            rows = np.flatnonzero(allowed)
            if rows.size == 0:
                return []
            scores = self._vectors.scores(vec, size)[rows]
            if scores.size > n:
                keep = scores >= np.partition(scores, -n)[-n]
                rows, scores = rows[keep], scores[keep]
            row_list = rows.tolist()
            ids = self._ids_for_rows(row_list)
        ranked = sorted(zip(row_list, scores.tolist(), strict=True), key=lambda p: (-p[1], ids[p[0]]))
        return [VectorHit(id=ids[row], score=score) for row, score in ranked[:n]]
```

- [ ] **Step 4: Run the tests**

Run: `uv run pytest tests/test_flat_search.py -v`
Expected: 13 passed.

- [ ] **Step 5: Gate and commit**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run pyright && uv run pytest -v`
Expected: all pass.

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(store): exact CPU vector search with filters and id tie-break"
```

### Task 10: Crash safety, store docs and phase 2 gate

**Files:**
- Test: `sidecar/tests/test_flat_crash.py`
- Modify: `docs/ARCHITECTURE.md`, `docs/STATUS.md`

**Interfaces:** Consumes `FlatStore.upsert`, `get`, `count`, `search_vectors`. Produces the verified guarantee: after a failure or hard kill between the vector write and the SQLite commit, the store reopens, never returns the half-written chunk, and keeps working.

- [ ] **Step 1: Write the tests**

`sidecar/tests/test_flat_crash.py`:

```python
import subprocess
import sys
import textwrap

import pytest
from helpers import DIM, make_chunk, unit_vectors

from omni_memory.store import FlatStore


def test_failed_metadata_write_leaves_a_consistent_store(tmp_path, monkeypatch):
    store = FlatStore(tmp_path / "idx", model_id="fake", dim=DIM)
    kept = make_chunk("kept")
    store.upsert([kept], unit_vectors(1))

    def explode(self, chunks, rows):
        raise RuntimeError("disk on fire")

    with monkeypatch.context() as patch:
        patch.setattr(FlatStore, "_write_metadata", explode)
        lost = make_chunk("lost")
        with pytest.raises(RuntimeError, match="disk on fire"):
            store.upsert([lost], unit_vectors(1, seed=1))

    assert store.count() == 1
    assert store.get([lost.id]) == {}
    assert {h.id for h in store.search_vectors(unit_vectors(1, seed=1)[0], 10)} == {kept.id}
    store.upsert([lost], unit_vectors(1, seed=1))  # the orphaned row is reusable
    assert store.count() == 2
    store.close()


def test_hard_kill_between_vector_write_and_commit(tmp_path):
    directory = tmp_path / "idx"
    script = textwrap.dedent(
        f"""
        import os
        import sys
        from pathlib import Path

        import numpy as np

        from omni_memory.store import FlatStore
        from omni_memory.types import Chunk, ChunkKind

        def chunk(name):
            return Chunk("s", name + ".txt", ChunkKind.TEXT, "{{}}", name, 1, name)

        vec = np.zeros((1, {DIM}), dtype=np.float32)
        vec[0, 0] = 1.0
        store = FlatStore(Path(sys.argv[1]), model_id="fake", dim={DIM})
        store.upsert([chunk("committed")], vec)
        FlatStore._write_metadata = lambda self, chunks, rows: os._exit(13)
        store.upsert([chunk("in-flight")], vec)
        """
    )
    result = subprocess.run(
        [sys.executable, "-I", "-c", script, str(directory)], capture_output=True, text=True
    )
    assert result.returncode == 13, result.stderr

    store = FlatStore(directory, model_id="fake", dim=DIM)
    assert store.count() == 1
    # exactly one result proves the in-flight vector row is ignored
    (survivor,) = store.search_vectors(unit_vectors(1)[0], 10)
    (kept,) = store.get([survivor.id]).values()
    assert kept.uri == "committed.txt"
    store.upsert([make_chunk("after")], unit_vectors(1, seed=4))  # and the store keeps working
    assert store.count() == 2
    store.close()
```

- [ ] **Step 2: Run the tests**

Run: `uv run pytest tests/test_flat_crash.py -v`
Expected: 2 passed. If the subprocess cannot import `omni_memory`, run `uv sync` (the editable install provides it) and retry; do not add `PYTHONPATH` hacks.

- [ ] **Step 3: Phase 2 docs gate**

`docs/ARCHITECTURE.md`: set `store/` to `built`; replace the "On-disk format" section body with:

```markdown
Index directory `%LOCALAPPDATA%\Omni\memory\` (tests and tools use `OMNI_MEMORY_DIR`):

- `meta.db` (SQLite, WAL, `synchronous=FULL`), schema version 1: `meta(key, value)` holds `schema_version`,
  `model_id`, `dim` (opening with different values raises `IndexMismatchError`: reindex required);
  `chunks(rid, id, vec_row, source_id, uri, kind, locator, content_hash, mtime_ns, text, identifiers)`;
  `chunks_fts` (FTS5 external-content over `text` and `identifiers`, kept in sync by triggers).
  Never `VACUUM` without checking the FTS mapping (it is keyed by `rid`, which is VACUUM-safe).
- `vectors.f32`: 64-byte header (`OMNIVEC1`, version, dim, capacity) then `capacity` rows of `dim`
  float32, row r at byte 64 + r*dim*4. Grows by doubling; freed rows are reused lowest-first.

Write order for a batch: vectors written and flushed, then one SQLite transaction. A vector row with
no `chunks` entry is garbage. Search is one float32 matrix multiply on the CPU (4.3 ms at 100k, 47 ms
at 1M measured in ADR 0001), ties broken by chunk id.
```

`docs/STATUS.md`: set phase 2 to `done`; under "Next" write "Plan phases 3-5 (ingestion, search + eval, API) using what phases 0-2 taught us."; add the measured GPU-vs-CPU cosine from Task 5 under "Known gaps".

- [ ] **Step 4: Final gate for the plan**

Run: `uv run ruff check . && uv run ruff format --check . && uv run pyright && uv run pytest -v`
Run: `uv run pytest -m model -v` (model tests; takes about a minute)
Expected: everything passes. Then verify nothing sensitive is staged: `git status --short` shows no `Sample Data/` or `.env` paths.

- [ ] **Step 5: Commit**

```bash
git add sidecar/tests docs
git commit -m "test(store): crash safety; docs: store format and phase 2 status"
```

---

## Self-Review (done against the spec)

**Spec coverage (phases 0-2).** Phase 0: skeleton, `Settings`, ruff/pyright/pytest, docs scaffold with `AGENTS.md` pointers (Tasks 1-3). Phase 1: `Embedder` protocol, `FakeEmbedder`, `GemmaEmbedder` text + image on bf16 GPU with CPU fallback, VRAM <= 1.5 GB gate, CPU/GPU gap measured (Tasks 4-5). Phase 2: schema with FTS5 and identifiers column, `VectorFile` with header and doubling, `FlatStore` exact CPU search, free-row reuse, tie-break by id, model/dimension mismatch error, write ordering, oracle tests, kill-between-writes test (Tasks 6-10). Spec items deliberately moved: `sources`/`files` tables and change detection (phase 3), `Store` protocol (phase 4), keyword query building (phase 4).

**Placeholders.** None: every step has its code or exact edit. The one value measured at run time (GPU/CPU cosine) is recorded in docs, not assumed.

**Type consistency.** `Chunk.id` uses `chunk_id(source_id, uri, locator, content_hash)` everywhere; `FlatStore` methods named `upsert`, `delete`, `get`, `count`, `close`, `search_vectors` are used identically in Tasks 8-10; `VectorFile` methods (`ensure_capacity`, `write_rows`, `read`, `scores`, `flush`, `close`) match between Task 7 and Task 8; `Filters` fields match Task 3. Test names and expected counts were re-checked against the test code.
