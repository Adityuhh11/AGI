# Memory Engine, Phases 3-4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Index real folders incrementally and safely (phase 3), then search them with vector + keyword fusion and measure retrieval quality against a golden query set (phase 4).

**Architecture:** `sources/` decides what may be read (secret deny list, `.gitignore`, file routing) and walks folders; `chunking/` turns files into chunks (text windows, markdown sections, PDF pages as text and image, images); `Indexer` diffs a folder against the store's `files` table and embeds only what changed, batching text across files; `FlatStore` gains source/file bookkeeping and FTS keyword search; `Searcher` fuses vector and keyword rankings with reciprocal rank fusion; `eval/` runs a golden query set against a real index and compares to a recorded baseline.

**Tech Stack:** Python >= 3.12, numpy, sqlite3 (FTS5), pathspec (gitignore), pypdfium2 (PDF), Pillow, PyYAML, threadpoolctl, sentence-transformers (existing), pytest, ruff, pyright.

**Spec:** `docs/superpowers/specs/2026-10-12-memory-engine-design.md` (phases 3 and 4 of section 13; sections 6, 7, 9, 10, 11). Previous plan: `docs/superpowers/plans/2026-10-12-memory-engine-phases-0-2.md` (merged). Phase 5 (HTTP API) is planned after this plan runs.

## Global Constraints

- Python `>=3.12`, `uv`; run commands from `C:\dev\VS_CODE\AGI\sidecar` unless a step says otherwise. Run Bash tool calls one at a time (parallel calls interleave the working directory).
- GPU = models only; index storage and search stay on the CPU. Embedding model `google/embeddinggemma-2`, 256-d float32 unit vectors.
- **Secrets are never read.** Built-in deny list that cannot be overridden: directories `.git`, `node_modules`, `.venv`, `venv`, `__pycache__`, `.ssh`; files `.env*`, `*.pem`, `*.key`, `id_rsa*`, `id_dsa*`, `id_ecdsa*`, `id_ed25519*`, `*.p12`, `*.pfx`, `*.jks`, `*.keystore`, `.netrc`, `.npmrc`, and lockfiles (`package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `uv.lock`, `poetry.lock`, `Pipfile.lock`, `Cargo.lock`, `go.sum`, `composer.lock`). Case-insensitive. Denied entries are counted, never opened.
- Size limits: text/code files 5 MB; images and PDFs 50 MB. Larger files are skipped and counted.
- Chunking baseline: 40-line windows with 10-line overlap; embedded/indexed text is `"<relative uri>\n<chunk body>"` capped at 6000 characters (the model also caps at 512 tokens); Markdown splits by heading then by size; PDF = one `pdf_text` chunk per non-empty page (text kept) plus one `pdf_page` chunk per page (image, rendered at 1.5x); images thumbnailed to max side 1024 with EXIF orientation applied.
- Incremental sync: `(size, mtime_ns)` first, SHA-256 only when those differ; an unchanged folder does zero embedding work; links (symlinks/junctions) are never followed; a directory that cannot be read never causes its indexed files to be deleted.
- Search: query embedded with the `SearchQuery` prompt; exact vector top-N and FTS5 bm25 top-N (N = 200) with the same filters; reciprocal rank fusion `score = sum(1 / (60 + rank))`; ties broken by chunk id; default k = 10.
- Eval: runs on `Sample Data/` (git-ignored); BLAS threads pinned to 1; a metric more than 0.02 below `docs/eval/baseline.json` fails; any `forbid` hit fails.
- Determinism: sorted directory walks, content-derived ids, injected clocks, seeded randomness, no GPU or real model in default tests (`FakeEmbedder`); real-model tests carry `@pytest.mark.model`, the 100k performance test `@pytest.mark.perf`.
- Quality gate for every task: `uv run ruff check .`, `uv run ruff format --check .`, `uv run pyright`, `uv run pytest` all clean before commit (check real exit status; do not hide it behind `| tail`).
- Conventional commits, one logical change each, tests with the code they cover, docs in the same commit as the behaviour. End every commit message with the `Co-Authored-By` trailer the harness specifies.
- Never read, index, log, print or commit `.env*` files or anything under `Sample Data/`.

## Review Focus

Inputs the spec implies but its stated tests do not exercise; each has a test in the owning task.

1. Secrets: a `.env` file (and key files) is never opened, even when it sits next to indexed files and even in the real sample repo; its contents are not searchable (Tasks 2, 6, 7).
2. One bad file never sinks a sync: a file that vanishes or is locked between scan and read, a corrupt PDF and an undecodable image are recorded as errors while everything else is indexed; an unreadable directory never deletes its previously indexed files (Tasks 2, 6).
3. Hostile file content: empty file, whitespace-only, binary data with a code extension, invalid UTF-8, CRLF, and a 1 MB single-line file all produce sane chunks or a clean skip (Tasks 3, 4).
4. Odd queries: empty, whitespace-only, punctuation-only, stopword-only, FTS operators and quotes, and a 5000-character query return results or `[]` without raising (Tasks 8, 9).
5. Windows paths: spaces and non-ASCII names work and URIs always use `/`; symlinks are not followed; a missing source root is an error, not an empty success (Tasks 2, 6).

## File Structure

```
sidecar/
  pyproject.toml                          (modify: deps, `perf` marker)
  eval/golden.yaml                        (create: 38 golden queries)
  src/agi_memory/
    config.py                             (modify: limits)
    types.py                              (modify: FileRecord, KeywordHit, SearchHit)
    sources/__init__.py  rules.py  folder.py
    chunking/__init__.py  common.py  text.py  pdf.py  image.py
    store/schema.py                       (modify: v2 tables)
    store/catalog.py                      (create: sources/files SQL)
    store/filters.py  keyword.py          (create)
    store/flat.py                         (modify)
    indexer.py                            (create)
    search/__init__.py  fusion.py  searcher.py
    eval/__init__.py  golden.py  metrics.py  runner.py  __main__.py
  tests/
    helpers.py                            (modify: file-chunk helpers)
    pdf_fixture.py                        (create)
    test_rules.py test_scan.py test_chunk_text.py test_chunk_pdf.py test_chunk_image.py
    test_catalog.py test_indexer.py test_sample_data.py
    test_keyword.py test_fusion.py test_searcher.py test_eval.py test_perf.py
docs/ARCHITECTURE.md  docs/STATUS.md  docs/eval/{baseline.json, <date>-baseline.md, perf.md}
```

Deviations from the spec made while planning (the plan updates the spec text in the task named): image chunks keep their file name as `text` so file names are keyword-searchable (spec said `''`); `pdf_page` chunks keep `''` (Task 4); `jobs.py` (background wrapper) moves to phase 5 with the API, phase 3 delivers a synchronous `Indexer.sync` with a progress callback; the `sources`/`files` schema is version 2 and v1 indexes are refused (there is no v1 data in the wild); sources yield file records, not `Document` objects.

---

## Phase 3: Ingestion

### Task 1: Dependencies, settings limits, secret deny list and file routing

**Files:**
- Modify: `sidecar/pyproject.toml`, `sidecar/src/agi_memory/config.py`
- Create: `sidecar/src/agi_memory/sources/__init__.py`, `sidecar/src/agi_memory/sources/rules.py`
- Test: `sidecar/tests/test_config.py` (append), `sidecar/tests/test_rules.py`

**Interfaces:**
- Produces `Settings` fields (all with defaults): `max_text_file_bytes=5*1024*1024`, `max_media_file_bytes=50*1024*1024`, `chunk_lines=40`, `chunk_overlap_lines=10`, `max_chunk_chars=6000`, `pdf_render_scale=1.5`, `image_max_side=1024`, `flush_chunks=64`; `Settings.__post_init__` raises `ValueError` mentioning `chunk_overlap_lines` unless `0 <= chunk_overlap_lines < chunk_lines`.
- Produces `agi_memory.sources.rules`: `Route` (`enum.StrEnum`: `CODE`, `MARKDOWN`, `TEXT`, `PDF`, `IMAGE`), `is_denied_dir(name: str) -> bool`, `is_denied_file(name: str) -> bool`, `is_denied(uri: str) -> bool` (any path component a denied dir, or the last a denied file), `route_for(name: str) -> Route | None` (by lower-cased suffix), `size_limit(route: Route, settings: Settings) -> int`.

- [ ] **Step 1: Write the failing tests**

Append to `sidecar/tests/test_config.py`:

```python
def test_limits_match_the_spec(tmp_path):
    s = Settings(index_dir=tmp_path)
    assert (s.max_text_file_bytes, s.max_media_file_bytes) == (5 * 1024 * 1024, 50 * 1024 * 1024)
    assert (s.chunk_lines, s.chunk_overlap_lines, s.max_chunk_chars) == (40, 10, 6000)
    assert (s.pdf_render_scale, s.image_max_side, s.flush_chunks) == (1.5, 1024, 64)


@pytest.mark.parametrize("overlap", [-1, 40, 41])
def test_overlap_must_be_smaller_than_the_window(tmp_path, overlap):
    with pytest.raises(ValueError, match="chunk_overlap_lines"):
        Settings(index_dir=tmp_path, chunk_lines=40, chunk_overlap_lines=overlap)
```

Create `sidecar/tests/test_rules.py`:

```python
import pytest

from agi_memory.config import Settings
from agi_memory.sources.rules import Route, is_denied, route_for, size_limit

DENIED = [
    ".env",
    "Backend/.env",
    "Frontend/.env.local",
    "Backend/.envsample",
    ".ENV",
    "keys/server.pem",
    "keys/tls.KEY",
    "home/id_rsa",
    "home/id_rsa.pub",
    "home/id_ed25519",
    "cert.p12",
    "cert.pfx",
    "release.keystore",
    ".netrc",
    "package-lock.json",
    "web/yarn.lock",
    "uv.lock",
    "go.sum",
    "node_modules/left-pad/index.js",
    "a/b/node_modules/x.js",
    ".git/config",
    "src/.git/HEAD",
    "pkg/__pycache__/x.pyc",
    ".venv/Lib/x.py",
    "x/.ssh/config",
]
ALLOWED = [
    "README.md",
    "src/environment.js",
    "src/keyboard.js",
    "docs/monkey.md",
    "Backend/src/utils/jwt.js",
    "notes/envelope.txt",
    "src/gitignore-notes.md",
    "Week 10 Prep.pdf",
]


@pytest.mark.parametrize("uri", DENIED)
def test_secret_and_noise_paths_are_denied(uri):
    assert is_denied(uri)


@pytest.mark.parametrize("uri", ALLOWED)
def test_ordinary_paths_are_allowed(uri):
    assert not is_denied(uri)


@pytest.mark.parametrize(
    ("name", "route"),
    [
        ("app.js", Route.CODE),
        ("App.JSX", Route.CODE),
        ("main.go", Route.CODE),
        ("style.css", Route.CODE),
        ("README.md", Route.MARKDOWN),
        ("notes.markdown", Route.MARKDOWN),
        ("data.csv", Route.TEXT),
        ("package.json", Route.TEXT),
        ("report.pdf", Route.PDF),
        ("Photo.JPG", Route.IMAGE),
        ("logo.png", Route.IMAGE),
        ("tool.exe", None),
        ("noextension", None),
        (".gitignore", None),
    ],
)
def test_routing_by_extension(name, route):
    assert route_for(name) is route


def test_size_limits_depend_on_the_route(tmp_path):
    settings = Settings(index_dir=tmp_path)
    for route in (Route.CODE, Route.MARKDOWN, Route.TEXT):
        assert size_limit(route, settings) == settings.max_text_file_bytes
    for route in (Route.PDF, Route.IMAGE):
        assert size_limit(route, settings) == settings.max_media_file_bytes
```

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_config.py tests/test_rules.py -q`
Expected: FAIL (`AttributeError`/`TypeError` for the new `Settings` fields; `ModuleNotFoundError: agi_memory.sources`).

- [ ] **Step 3: Update `sidecar/pyproject.toml`**

Replace the `dependencies = [...]` block with:

```toml
dependencies = [
    "numpy",
    "pathspec",
    "pillow",
    "pypdfium2",
    "pyyaml",
    "sentence-transformers[image]>=6.1.0",
    "threadpoolctl",
    "torch",
    "transformers",
]
```

and replace the two pytest lines under `[tool.pytest.ini_options]` that set `addopts` and `markers` with:

```toml
addopts = "-ra --strict-markers -m 'not model and not perf'"
markers = [
    "model: needs the real embedding model (and a GPU); run with -m model",
    "perf: slow performance checks on 100k chunks; run with -m perf",
]
```

Run: `uv sync`
Expected: installs pathspec, pypdfium2, pyyaml, threadpoolctl; the lockfile changes.

- [ ] **Step 4: Implement `Settings` limits**

In `sidecar/src/agi_memory/config.py`, add these fields after `image_batch_size` in `Settings`:

```python
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
```

(Place `__post_init__` before `from_env`, after the fields.)

- [ ] **Step 5: Implement `sources/rules.py`**

Create `sidecar/src/agi_memory/sources/__init__.py`:

```python
"""Where indexed files come from: the deny list, file routing and the folder scanner."""
```

Create `sidecar/src/agi_memory/sources/rules.py`:

```python
"""What the indexer may read: the secret deny list and the file-type routing table.

The deny list is not configurable on purpose. Anything that can hold a credential, plus
dependency and VCS directories, is skipped before it is ever opened.
"""

from __future__ import annotations

import enum
import fnmatch
from pathlib import PurePosixPath

from agi_memory.config import Settings


class Route(enum.StrEnum):
    """How a file is turned into chunks."""

    CODE = "code"
    MARKDOWN = "markdown"
    TEXT = "text"
    PDF = "pdf"
    IMAGE = "image"


_DENIED_DIRS = frozenset({".git", "node_modules", ".venv", "venv", "__pycache__", ".ssh"})
_DENIED_FILE_PATTERNS = (
    ".env*",
    "*.pem",
    "*.key",
    "id_rsa*",
    "id_dsa*",
    "id_ecdsa*",
    "id_ed25519*",
    "*.p12",
    "*.pfx",
    "*.jks",
    "*.keystore",
    ".netrc",
    ".npmrc",
    "package-lock.json",
    "yarn.lock",
    "pnpm-lock.yaml",
    "uv.lock",
    "poetry.lock",
    "pipfile.lock",
    "cargo.lock",
    "go.sum",
    "composer.lock",
)

_CODE = frozenset(
    ".js .jsx .mjs .cjs .ts .tsx .py .go .rs .java .kt .c .h .cpp .hpp .cs .rb .php .sh .ps1 "
    ".bat .sql .css .scss .html .htm .svelte .vue .lua .swift".split()
)
_MARKDOWN = frozenset({".md", ".markdown"})
_TEXT = frozenset(".txt .csv .tsv .json .yaml .yml .toml .ini .cfg .xml .rst .tex".split())
_IMAGE = frozenset({".png", ".jpg", ".jpeg", ".webp", ".bmp", ".gif"})


def is_denied_dir(name: str) -> bool:
    return name.lower() in _DENIED_DIRS


def is_denied_file(name: str) -> bool:
    lowered = name.lower()
    return any(fnmatch.fnmatchcase(lowered, pattern) for pattern in _DENIED_FILE_PATTERNS)


def is_denied(uri: str) -> bool:
    """True if any directory on the path, or the file name itself, is on the deny list."""
    parts = uri.split("/")
    return any(is_denied_dir(part) for part in parts) or is_denied_file(parts[-1])


def route_for(name: str) -> Route | None:
    """How to chunk a file with this name, or None if it is not a supported type."""
    suffix = PurePosixPath(name).suffix.lower()
    if suffix in _CODE:
        return Route.CODE
    if suffix in _MARKDOWN:
        return Route.MARKDOWN
    if suffix in _TEXT:
        return Route.TEXT
    if suffix == ".pdf":
        return Route.PDF
    if suffix in _IMAGE:
        return Route.IMAGE
    return None


def size_limit(route: Route, settings: Settings) -> int:
    """Largest file (bytes) the route will index."""
    if route in (Route.PDF, Route.IMAGE):
        return settings.max_media_file_bytes
    return settings.max_text_file_bytes
```

- [ ] **Step 6: Run the gate**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass (52 new test cases: 4 config and 48 rules).

- [ ] **Step 7: Commit**

```bash
git add sidecar/pyproject.toml sidecar/uv.lock sidecar/src sidecar/tests
git commit -m "feat(sources): secret deny list, file routing and ingestion limits"
```

### Task 2: Folder scanner

**Files:**
- Create: `sidecar/src/agi_memory/sources/folder.py`
- Test: `sidecar/tests/test_scan.py`

**Interfaces:**
- Consumes: `Route`, `is_denied_dir`, `is_denied_file`, `route_for`, `size_limit` (Task 1); `Settings`.
- Produces `agi_memory.sources.folder`:
  - `ScannedFile(uri: str, path: Path, route: Route, size: int, mtime_ns: int)` frozen dataclass.
  - `ScanResult` dataclass with `files: list[ScannedFile]`, `denied: int`, `ignored: int`, `unsupported: int`, `too_large: int`, `unreadable: list[tuple[str, str]]` (uri prefix and message; a directory's prefix ends with `/`, the root's prefix is `""`).
  - `scan(root: Path, settings: Settings) -> ScanResult`: depth-first, entries sorted by name, URIs relative to `root` with `/`, honours nested `.gitignore` files (last matching rule wins, deeper files override), skips symlinks and junctions silently, counts a denied/ignored directory once.

- [ ] **Step 1: Write the failing tests**

`sidecar/tests/test_scan.py`:

```python
import os

import pytest

from agi_memory.config import Settings
from agi_memory.sources.folder import scan
from agi_memory.sources.rules import Route


def write(root, relative, content="x"):
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return path


@pytest.fixture
def settings(tmp_path):
    return Settings(index_dir=tmp_path / "idx")


def uris(result):
    return [f.uri for f in result.files]


def test_denied_entries_are_counted_and_never_listed(tmp_path, settings):
    root = tmp_path / "src"
    for name in (".env", ".git/config", "node_modules/pkg/index.js", "server.pem", "package-lock.json"):
        write(root, name)
    write(root, "ok.js")
    result = scan(root, settings)
    assert uris(result) == ["ok.js"]
    assert result.denied == 5  # three files plus two directories counted once each


def test_nested_gitignore_rules_apply_and_deeper_files_override(tmp_path, settings):
    root = tmp_path / "src"
    write(root, ".gitignore", "*.md\nbuild/\n/secret.txt\n")
    write(root, "sub/.gitignore", "!keep.md\n")
    for name in (
        "app.js", "notes.md", "secret.txt", "build/out.js", "sub/keep.md", "sub/other.md",
        "sub/deep/x.js", "other/secret.txt",
    ):  # fmt: skip
        write(root, name)
    result = scan(root, settings)
    assert uris(result) == ["app.js", "other/secret.txt", "sub/deep/x.js", "sub/keep.md"]
    assert result.ignored == 4  # notes.md, secret.txt, build/ (once), sub/other.md


def test_unsupported_and_oversized_files_are_counted(tmp_path):
    settings = Settings(index_dir=tmp_path / "idx", max_text_file_bytes=10, max_media_file_bytes=10)
    root = tmp_path / "src"
    write(root, "data.bin", "binary-ish")
    write(root, "small.txt", "tiny")
    write(root, "big.txt", "x" * 100)
    (root / "pic.png").write_bytes(b"x" * 20)
    result = scan(root, settings)
    assert uris(result) == ["small.txt"]
    assert (result.unsupported, result.too_large) == (1, 2)


def test_order_is_sorted_and_uris_use_forward_slashes_with_odd_names(tmp_path, settings):
    root = tmp_path / "src"
    for name in ("my notes/café.md", "a b.txt", "Z.js", "a.js"):
        write(root, name)
    result = scan(root, settings)
    assert uris(result) == ["Z.js", "a b.txt", "a.js", "my notes/café.md"]
    assert all("\\" not in f.uri for f in result.files)


def test_file_records_size_mtime_and_route(tmp_path, settings):
    root = tmp_path / "src"
    path = write(root, "a/b.md", "hello")
    (found,) = scan(root, settings).files
    stat = path.stat()
    assert (found.uri, found.route, found.size, found.mtime_ns) == (
        "a/b.md", Route.MARKDOWN, 5, stat.st_mtime_ns,
    )  # fmt: skip
    assert found.path == path


def test_symlinks_are_not_followed(tmp_path, settings):
    root = tmp_path / "src"
    outside = tmp_path / "outside"
    write(outside, "secret.txt", "do not index")
    write(root, "real.txt")
    try:
        os.symlink(outside, root / "link", target_is_directory=True)
        os.symlink(outside / "secret.txt", root / "file-link.txt")
    except OSError:
        pytest.skip("creating symlinks needs privileges on this machine")
    assert uris(scan(root, settings)) == ["real.txt"]


def test_unreadable_directory_is_reported_and_the_rest_still_scanned(tmp_path, settings, monkeypatch):
    root = tmp_path / "src"
    write(root, "ok.js")
    write(root, "locked/hidden.js")
    real_scandir = os.scandir

    def flaky(path):
        if str(path).endswith("locked"):
            raise PermissionError("access denied")
        return real_scandir(path)

    monkeypatch.setattr(os, "scandir", flaky)
    result = scan(root, settings)
    assert uris(result) == ["ok.js"]
    assert result.unreadable == [("locked/", "access denied")]


def test_missing_root_is_reported_as_unreadable_root(tmp_path, settings):
    result = scan(tmp_path / "nope", settings)
    assert result.files == []
    assert [prefix for prefix, _ in result.unreadable] == [""]
```

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_scan.py -q`
Expected: FAIL (`ModuleNotFoundError: agi_memory.sources.folder`).

- [ ] **Step 3: Implement `sources/folder.py`**

```python
"""Walks a folder and decides which files the indexer may touch.

Order is deterministic (entries sorted by name). Symlinks and junctions are never followed, so
a link cannot lead the indexer out of the root. Directories the OS refuses to list are recorded
in `unreadable` so the caller can avoid treating their files as deleted.
"""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from pathlib import Path

import pathspec

from agi_memory.config import Settings
from agi_memory.sources.rules import (
    Route,
    is_denied_dir,
    is_denied_file,
    route_for,
    size_limit,
)

_Specs = tuple[tuple[str, pathspec.GitIgnoreSpec], ...]  # (uri prefix the file applies under, rules)


@dataclass(frozen=True, slots=True)
class ScannedFile:
    uri: str  # path relative to the root, '/'-separated
    path: Path
    route: Route
    size: int
    mtime_ns: int


@dataclass(slots=True)
class ScanResult:
    files: list[ScannedFile] = field(default_factory=list)
    denied: int = 0  # deny-list hits (a directory counts once)
    ignored: int = 0  # .gitignore hits (a directory counts once)
    unsupported: int = 0  # regular files of an unknown type
    too_large: int = 0
    unreadable: list[tuple[str, str]] = field(default_factory=list)  # (uri prefix, message)


def scan(root: Path, settings: Settings) -> ScanResult:
    """List the indexable files under `root`."""
    result = ScanResult()
    _visit(root, "", (), settings, result)
    return result


def _with_local_gitignore(directory: Path, prefix: str, specs: _Specs) -> _Specs:
    try:
        lines = (directory / ".gitignore").read_text(encoding="utf-8", errors="replace")
    except OSError:  # no .gitignore here (or unreadable): its rules simply do not apply
        return specs
    return (*specs, (prefix, pathspec.GitIgnoreSpec.from_lines(lines.splitlines())))


def _gitignored(uri: str, specs: _Specs) -> bool:
    ignored = False
    for base, spec in specs:
        verdict = spec.check_file(uri[len(base) :]).include
        if verdict is not None:  # deeper files are visited later, so they override
            ignored = verdict
    return ignored


def _visit(directory: Path, prefix: str, specs: _Specs, settings: Settings, result: ScanResult) -> None:
    specs = _with_local_gitignore(directory, prefix, specs)
    try:
        with os.scandir(directory) as iterator:
            entries = sorted(iterator, key=lambda entry: entry.name)
    except OSError as error:
        result.unreadable.append((prefix, str(error)))
        return
    for entry in entries:
        if entry.is_symlink() or entry.is_junction():
            continue  # never follow a link out of the root
        uri = prefix + entry.name
        if entry.is_dir(follow_symlinks=False):
            if is_denied_dir(entry.name):
                result.denied += 1
            elif _gitignored(uri + "/", specs):
                result.ignored += 1
            else:
                _visit(Path(entry.path), uri + "/", specs, settings, result)
        elif entry.is_file(follow_symlinks=False):
            _consider_file(entry, uri, specs, settings, result)


def _consider_file(
    entry: os.DirEntry[str], uri: str, specs: _Specs, settings: Settings, result: ScanResult
) -> None:
    if is_denied_file(entry.name):
        result.denied += 1
        return
    if _gitignored(uri, specs):
        result.ignored += 1
        return
    route = route_for(entry.name)
    if route is None:
        result.unsupported += 1
        return
    try:
        stat = entry.stat(follow_symlinks=False)
    except OSError as error:
        result.unreadable.append((uri, str(error)))
        return
    if stat.st_size > size_limit(route, settings):
        result.too_large += 1
        return
    result.files.append(ScannedFile(uri, Path(entry.path), route, stat.st_size, stat.st_mtime_ns))
```

- [ ] **Step 4: Run the gate**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass (8 new tests; the symlink test passes or skips). If `entry.is_junction` is flagged by pyright, check the installed Python is 3.12+ (it is 3.13) rather than ignoring.

- [ ] **Step 5: Commit**

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(sources): gitignore-aware folder scanner that never follows links"
```

### Task 3: Text, code and Markdown chunkers

**Files:**
- Create: `sidecar/src/agi_memory/chunking/__init__.py`, `sidecar/src/agi_memory/chunking/common.py`, `sidecar/src/agi_memory/chunking/text.py`
- Test: `sidecar/tests/test_chunk_text.py`

**Interfaces:**
- Consumes: `Chunk`, `ChunkKind` (types); `Route` (Task 1); `Settings` (Task 1 fields).
- Produces `agi_memory.chunking.common`: `ChunkingError(Exception)`, `locator_json(**fields: object) -> str` (canonical: sorted keys, no spaces), `sha256_hex(text: str) -> str`.
- Produces `agi_memory.chunking.text`: `NotTextError(ChunkingError)` (file looks binary), `chunk_text_document(source_id: str, uri: str, route: Route, data: bytes, mtime_ns: int, settings: Settings) -> list[Chunk]` for routes `CODE`, `MARKDOWN`, `TEXT` (other routes raise `ValueError`). Each chunk: `text = f"{uri}\n{body}"[:max_chunk_chars]`, `content_hash = sha256_hex(text)`, locator `{"lines":[first,last]}` (1-based, inclusive), kind from the route; blank bodies are dropped.

- [ ] **Step 1: Write the failing tests**

`sidecar/tests/test_chunk_text.py`:

```python
import hashlib
import json

import pytest

from agi_memory.chunking.common import locator_json
from agi_memory.chunking.text import NotTextError, chunk_text_document
from agi_memory.config import Settings
from agi_memory.sources.rules import Route
from agi_memory.types import ChunkKind


@pytest.fixture
def settings(tmp_path):
    return Settings(index_dir=tmp_path)


def chunk(settings, route, text, uri="src/a.js"):
    return chunk_text_document("repo", uri, route, text.encode(), 7, settings)


def numbered(count):
    return "\n".join(f"line {i}" for i in range(1, count + 1))


def line_ranges(chunks):
    return [json.loads(c.locator)["lines"] for c in chunks]


def test_code_windows_overlap_by_ten_lines(settings):
    chunks = chunk(settings, Route.CODE, numbered(100))
    assert line_ranges(chunks) == [[1, 40], [31, 70], [61, 100]]
    assert all(c.kind is ChunkKind.CODE for c in chunks)


def test_short_file_is_one_chunk_and_exact_window_is_not_split(settings):
    assert line_ranges(chunk(settings, Route.CODE, numbered(40))) == [[1, 40]]
    assert line_ranges(chunk(settings, Route.CODE, numbered(41))) == [[1, 40], [31, 41]]


def test_chunk_text_starts_with_the_uri_and_metadata_is_carried(settings):
    first = chunk(settings, Route.CODE, numbered(3))[0]
    assert first.text == "src/a.js\nline 1\nline 2\nline 3"
    assert first.locator == '{"lines":[1,3]}'
    assert (first.source_id, first.uri, first.mtime_ns) == ("repo", "src/a.js", 7)
    assert first.content_hash == hashlib.sha256(first.text.encode()).hexdigest()


def test_ids_are_deterministic_and_differ_per_window(settings):
    a = chunk(settings, Route.CODE, numbered(100))
    b = chunk(settings, Route.CODE, numbered(100))
    assert [c.id for c in a] == [c.id for c in b]
    assert len({c.id for c in a}) == 3


@pytest.mark.parametrize("content", ["", "   \n\n\t\n"])
def test_empty_and_blank_files_produce_no_chunks(settings, content):
    assert chunk(settings, Route.TEXT, content) == []


def test_crlf_line_endings_are_normalised(settings):
    (only,) = chunk(settings, Route.TEXT, "a\r\nb\r\n", uri="x.txt")
    assert only.text == "x.txt\na\nb"


def test_binary_data_is_not_text(settings):
    with pytest.raises(NotTextError):
        chunk_text_document("repo", "a.js", Route.CODE, b"\x00\x01\x02binary", 1, settings)


def test_invalid_utf8_is_replaced_not_fatal(settings):
    (only,) = chunk_text_document("repo", "a.txt", Route.TEXT, b"ok \xff\xfe bad", 1, settings)
    assert "\ufffd" in only.text


def test_one_enormous_line_is_capped(settings):
    (only,) = chunk(settings, Route.CODE, "x" * 1_000_000)
    assert len(only.text) == settings.max_chunk_chars


def test_text_route_gives_text_kind_and_other_routes_are_rejected(settings):
    assert chunk(settings, Route.TEXT, "a,b\n1,2", uri="d.csv")[0].kind is ChunkKind.TEXT
    with pytest.raises(ValueError, match="route"):
        chunk(settings, Route.PDF, "x")


def test_markdown_splits_by_heading(settings):
    doc = "# Title\nintro\n## A\nalpha\n## B\nbeta"
    chunks = chunk(settings, Route.MARKDOWN, doc, uri="r.md")
    assert line_ranges(chunks) == [[1, 2], [3, 4], [5, 6]]
    assert all(c.kind is ChunkKind.MARKDOWN for c in chunks)


def test_markdown_ignores_hashes_inside_code_fences(settings):
    doc = "# T\n```\n# not a heading\n```\ntext"
    assert line_ranges(chunk(settings, Route.MARKDOWN, doc, uri="r.md")) == [[1, 5]]


def test_markdown_text_before_the_first_heading_is_kept(settings):
    doc = "preamble\n# H\nbody"
    assert line_ranges(chunk(settings, Route.MARKDOWN, doc, uri="r.md")) == [[1, 1], [2, 3]]


def test_a_long_markdown_section_is_windowed(settings):
    doc = "# H\n" + numbered(100)  # 101 lines in one section
    ranges = line_ranges(chunk(settings, Route.MARKDOWN, doc, uri="r.md"))
    assert ranges == [[1, 40], [31, 70], [61, 100], [91, 101]]


def test_locator_json_is_canonical():
    assert locator_json(page=3, extra=None) == '{"extra":null,"page":3}'
```

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_chunk_text.py -q`
Expected: FAIL (`ModuleNotFoundError: agi_memory.chunking`).

- [ ] **Step 3: Implement**

`sidecar/src/agi_memory/chunking/__init__.py`:

```python
"""Turning files into chunks: text/code/Markdown windows, PDF pages, images."""
```

`sidecar/src/agi_memory/chunking/common.py`:

```python
"""Small helpers shared by every chunker."""

from __future__ import annotations

import hashlib
import json


class ChunkingError(Exception):
    """A file could not be turned into chunks (corrupt, encrypted, undecodable...)."""


def locator_json(**fields: object) -> str:
    """Canonical JSON for a chunk locator: sorted keys, no spaces, so equal inputs give equal ids."""
    return json.dumps(fields, separators=(",", ":"), sort_keys=True)


def sha256_hex(text: str) -> str:
    return hashlib.sha256(text.encode()).hexdigest()
```

`sidecar/src/agi_memory/chunking/text.py`:

```python
"""Chunks for text-like files: source code, plain text, CSV/JSON and Markdown.

Baseline strategy (measured later against the golden set): fixed windows of lines with overlap,
Markdown split by heading first. Every chunk's text starts with the file's relative path so the
path is both embedded and keyword-searchable.
"""

from __future__ import annotations

import re
from collections.abc import Iterator

from agi_memory.chunking.common import ChunkingError, locator_json, sha256_hex
from agi_memory.config import Settings
from agi_memory.sources.rules import Route
from agi_memory.types import Chunk, ChunkKind

_KINDS = {
    Route.CODE: ChunkKind.CODE,
    Route.MARKDOWN: ChunkKind.MARKDOWN,
    Route.TEXT: ChunkKind.TEXT,
}
_HEADING = re.compile(r"^#{1,6}\s")
_BINARY_PROBE = 8192


class NotTextError(ChunkingError):
    """The file looks binary (NUL bytes), so it is skipped rather than indexed as garbage."""


def chunk_text_document(
    source_id: str, uri: str, route: Route, data: bytes, mtime_ns: int, settings: Settings
) -> list[Chunk]:
    """Chunks for one text-like file (`route` must be CODE, MARKDOWN or TEXT)."""
    kind = _KINDS.get(route)
    if kind is None:
        raise ValueError(f"route {route!r} is not a text route")
    if b"\0" in data[:_BINARY_PROBE]:
        raise NotTextError(f"{uri} looks binary")
    lines = data.decode("utf-8", errors="replace").splitlines()
    if route is Route.MARKDOWN:
        spans = _markdown_spans(lines, settings)
    else:
        spans = list(_windows(0, len(lines), settings))
    chunks = []
    for first, last in spans:
        body = "\n".join(lines[first - 1 : last])
        if not body.strip():
            continue
        text = f"{uri}\n{body}"[: settings.max_chunk_chars]
        chunks.append(
            Chunk(
                source_id=source_id,
                uri=uri,
                kind=kind,
                locator=locator_json(lines=[first, last]),
                content_hash=sha256_hex(text),
                mtime_ns=mtime_ns,
                text=text,
            )
        )
    return chunks


def _windows(start: int, stop: int, settings: Settings) -> Iterator[tuple[int, int]]:
    """Overlapping windows over 0-based [start, stop), yielded as 1-based inclusive ranges."""
    step = settings.chunk_lines - settings.chunk_overlap_lines
    position = start
    while position < stop:
        end = min(position + settings.chunk_lines, stop)
        yield position + 1, end
        if end == stop:
            return
        position += step


def _markdown_spans(lines: list[str], settings: Settings) -> list[tuple[int, int]]:
    """One span per heading section; sections longer than a window are windowed."""
    bounds = [0]
    in_fence = False
    for index, line in enumerate(lines):
        if line.lstrip().startswith("```"):
            in_fence = not in_fence
        elif not in_fence and index > 0 and _HEADING.match(line):
            bounds.append(index)
    bounds.append(len(lines))
    spans: list[tuple[int, int]] = []
    for start, stop in zip(bounds, bounds[1:], strict=False):
        spans.extend(_windows(start, stop, settings))
    return spans
```

- [ ] **Step 4: Run the gate**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass (16 new test cases).

- [ ] **Step 5: Commit**

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(chunking): line-window and Markdown-section chunkers for text files"
```

### Task 4: PDF and image chunkers

**Files:**
- Create: `sidecar/src/agi_memory/chunking/pdf.py`, `sidecar/src/agi_memory/chunking/image.py`, `sidecar/tests/pdf_fixture.py`
- Test: `sidecar/tests/test_chunk_pdf.py`, `sidecar/tests/test_chunk_image.py`
- Modify: `docs/superpowers/specs/2026-10-12-memory-engine-design.md` (section 5/6 note on image `text`)

**Interfaces:**
- Consumes: `ChunkingError`, `locator_json`, `sha256_hex` (Task 3); `Chunk`, `ChunkKind`; `Settings`.
- Produces `agi_memory.chunking.pdf`:
  - `PdfPages(data: bytes)`: context manager; raises `ChunkingError` if pdfium cannot open the file; `len(pdf)` pages; `text(index: int) -> str` (0-based); `render(index: int, scale: float) -> PIL.Image.Image` (RGB); `close()`.
  - `pdf_text_chunks(source_id, uri, pdf: PdfPages, mtime_ns, settings) -> list[Chunk]`: kind `PDF_TEXT`, locator `{"page": n}` (1-based), one per page whose stripped text is non-empty, `text = f"{uri}\n{body}"[:max_chunk_chars]`.
  - `pdf_page_chunks(source_id, uri, file_sha256: str, page_count: int, mtime_ns) -> list[Chunk]`: kind `PDF_PAGE`, locator `{"page": n}`, `text=""`, `content_hash = sha256_hex(f"{file_sha256}:{n}")`.
- Produces `agi_memory.chunking.image`: `load_image(data: bytes, max_side: int) -> PIL.Image.Image` (EXIF-rotated, RGB, thumbnailed; raises `ChunkingError` on undecodable data), `image_chunk(source_id, uri, file_sha256: str, mtime_ns) -> Chunk` (kind `IMAGE`, locator `"{}"`, `text=uri`, `content_hash=file_sha256`).
- Produces test helper `tests/pdf_fixture.make_text_pdf(pages: list[str]) -> bytes`.

- [ ] **Step 1: Test fixture and failing tests**

`sidecar/tests/pdf_fixture.py`:

```python
def make_text_pdf(pages: list[str]) -> bytes:
    """A minimal valid PDF with one line of Helvetica text per page (pdfium can extract it)."""
    count = len(pages)
    kids = " ".join(f"{4 + 2 * i} 0 R" for i in range(count))
    objects = [
        b"<< /Type /Catalog /Pages 2 0 R >>",
        f"<< /Type /Pages /Kids [{kids}] /Count {count} >>".encode(),
        b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
    ]
    for index, text in enumerate(pages):
        safe = text.replace("\\", "\\\\").replace("(", "\\(").replace(")", "\\)")
        stream = f"BT /F1 18 Tf 50 700 Td ({safe}) Tj ET".encode()
        objects.append(
            f"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents {5 + 2 * index} 0 R "
            f"/Resources << /Font << /F1 3 0 R >> >> >>".encode()
        )
        objects.append(
            b"<< /Length " + str(len(stream)).encode() + b" >>\nstream\n" + stream + b"\nendstream"
        )
    out = bytearray(b"%PDF-1.4\n")
    offsets = []
    for number, body in enumerate(objects, start=1):
        offsets.append(len(out))
        out += f"{number} 0 obj\n".encode() + body + b"\nendobj\n"
    xref = len(out)
    out += f"xref\n0 {len(objects) + 1}\n".encode() + b"0000000000 65535 f \n"
    for offset in offsets:
        out += f"{offset:010d} 00000 n \n".encode()
    out += (
        f"trailer\n<< /Size {len(objects) + 1} /Root 1 0 R >>\nstartxref\n{xref}\n%%EOF\n"
    ).encode()
    return bytes(out)
```

`sidecar/tests/test_chunk_pdf.py`:

```python
import io

import pytest
from pdf_fixture import make_text_pdf
from PIL import Image

from agi_memory.chunking.common import ChunkingError
from agi_memory.chunking.pdf import PdfPages, pdf_page_chunks, pdf_text_chunks
from agi_memory.config import Settings
from agi_memory.types import ChunkKind


@pytest.fixture
def settings(tmp_path):
    return Settings(index_dir=tmp_path)


def image_only_pdf() -> bytes:
    buffer = io.BytesIO()
    Image.new("RGB", (200, 100), "white").save(buffer, format="PDF")
    return buffer.getvalue()


def test_text_pages_become_text_chunks(settings):
    data = make_text_pdf(["Hello PDF world", "Second page (with parens)"])
    with PdfPages(data) as pdf:
        assert len(pdf) == 2
        chunks = pdf_text_chunks("docs", "a/Report.pdf", pdf, 5, settings)
    assert [c.locator for c in chunks] == ['{"page":1}', '{"page":2}']
    assert all(c.kind is ChunkKind.PDF_TEXT for c in chunks)
    assert chunks[0].text.startswith("a/Report.pdf\nHello PDF world")
    assert "(with parens)" in chunks[1].text
    assert chunks[0].mtime_ns == 5


def test_page_image_chunks_are_keyed_by_file_hash_and_page(settings):
    first = pdf_page_chunks("docs", "a.pdf", "hash-one", 3, 5)
    again = pdf_page_chunks("docs", "a.pdf", "hash-one", 3, 5)
    other = pdf_page_chunks("docs", "a.pdf", "hash-two", 3, 5)
    assert [c.locator for c in first] == ['{"page":1}', '{"page":2}', '{"page":3}']
    assert all(c.kind is ChunkKind.PDF_PAGE and c.text == "" for c in first)
    assert [c.id for c in first] == [c.id for c in again]
    assert {c.id for c in first}.isdisjoint({c.id for c in other})


def test_a_scanned_pdf_has_no_text_chunks_but_still_renders(settings):
    with PdfPages(image_only_pdf()) as pdf:
        assert pdf_text_chunks("docs", "scan.pdf", pdf, 1, settings) == []
        image = pdf.render(0, 1.5)
    assert image.mode == "RGB"
    assert image.size[0] > 200  # a 200pt-wide page at 1.5x scale


def test_long_page_text_is_capped(settings):
    data = make_text_pdf(["word " * 5000])
    with PdfPages(data) as pdf:
        (chunk,) = pdf_text_chunks("docs", "long.pdf", pdf, 1, settings)
    assert len(chunk.text) == settings.max_chunk_chars


def test_corrupt_pdf_is_a_chunking_error():
    with pytest.raises(ChunkingError, match="PDF"):
        PdfPages(b"%PDF-1.4 this is not a pdf")
```

`sidecar/tests/test_chunk_image.py`:

```python
import io

import pytest
from PIL import Image

from agi_memory.chunking.common import ChunkingError
from agi_memory.chunking.image import image_chunk, load_image
from agi_memory.types import ChunkKind


def encode(image, fmt, **kwargs):
    buffer = io.BytesIO()
    image.save(buffer, format=fmt, **kwargs)
    return buffer.getvalue()


def test_large_images_are_thumbnailed_to_the_max_side():
    data = encode(Image.new("RGB", (3000, 2000), "red"), "PNG")
    image = load_image(data, 1024)
    assert max(image.size) == 1024
    assert image.mode == "RGB"


@pytest.mark.parametrize("mode", ["RGBA", "P", "L"])
def test_every_mode_is_converted_to_rgb(mode):
    data = encode(Image.new(mode, (20, 10)), "PNG")
    assert load_image(data, 1024).mode == "RGB"


def test_exif_orientation_is_applied():
    exif = Image.Exif()
    exif[0x0112] = 6  # rotate 90 degrees clockwise to display
    data = encode(Image.new("RGB", (100, 50), "blue"), "JPEG", exif=exif)
    assert load_image(data, 1024).size == (50, 100)


def test_undecodable_bytes_are_a_chunking_error():
    with pytest.raises(ChunkingError, match="image"):
        load_image(b"definitely not an image", 1024)


def test_image_chunk_keeps_the_file_name_searchable():
    chunk = image_chunk("pics", "trips/Rome 2025.png", "abc123", 9)
    assert chunk.kind is ChunkKind.IMAGE
    assert (chunk.text, chunk.locator, chunk.content_hash, chunk.mtime_ns) == (
        "trips/Rome 2025.png", "{}", "abc123", 9,
    )  # fmt: skip
```

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_chunk_pdf.py tests/test_chunk_image.py -q`
Expected: FAIL (`ModuleNotFoundError: agi_memory.chunking.pdf`).

- [ ] **Step 3: Implement `chunking/pdf.py`**

```python
"""PDF pages as text chunks and as image chunks, through pdfium.

Each page yields up to two chunks: `pdf_text` (the extracted text, searchable by keyword and by
meaning) and `pdf_page` (the rendered page, embedded as an image so tables, diagrams and scanned
pages are findable). Rendering is lazy: the caller renders pages in small batches so a 500-page
PDF never sits in memory as bitmaps.
"""

from __future__ import annotations

from types import TracebackType
from typing import Self

import pypdfium2 as pdfium
from PIL.Image import Image

from agi_memory.chunking.common import ChunkingError, locator_json, sha256_hex
from agi_memory.config import Settings
from agi_memory.types import Chunk, ChunkKind


class PdfPages:
    """An open PDF. Use as a context manager so the native handle is always released."""

    def __init__(self, data: bytes) -> None:
        try:
            self._pdf = pdfium.PdfDocument(data)
        except pdfium.PdfiumError as error:
            raise ChunkingError(f"unreadable PDF: {error}") from error

    def __enter__(self) -> Self:
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        traceback: TracebackType | None,
    ) -> None:
        self.close()

    def close(self) -> None:
        self._pdf.close()

    def __len__(self) -> int:
        return len(self._pdf)

    def text(self, index: int) -> str:
        """Extracted text of page `index` (0-based); empty for scanned pages."""
        try:
            page = self._pdf[index]
            try:
                textpage = page.get_textpage()
                try:
                    return textpage.get_text_range()
                finally:
                    textpage.close()
            finally:
                page.close()
        except pdfium.PdfiumError as error:
            raise ChunkingError(f"cannot read text of PDF page {index + 1}: {error}") from error

    def render(self, index: int, scale: float) -> Image:
        """Page `index` (0-based) as an RGB image."""
        try:
            page = self._pdf[index]
            try:
                return page.render(scale=scale).to_pil().convert("RGB")
            finally:
                page.close()
        except pdfium.PdfiumError as error:
            raise ChunkingError(f"cannot render PDF page {index + 1}: {error}") from error


def pdf_text_chunks(
    source_id: str, uri: str, pdf: PdfPages, mtime_ns: int, settings: Settings
) -> list[Chunk]:
    """One `pdf_text` chunk for every page that has any text."""
    chunks = []
    for index in range(len(pdf)):
        body = pdf.text(index).replace("\r\n", "\n").strip()
        if not body:
            continue
        text = f"{uri}\n{body}"[: settings.max_chunk_chars]
        chunks.append(
            Chunk(
                source_id=source_id,
                uri=uri,
                kind=ChunkKind.PDF_TEXT,
                locator=locator_json(page=index + 1),
                content_hash=sha256_hex(text),
                mtime_ns=mtime_ns,
                text=text,
            )
        )
    return chunks


def pdf_page_chunks(
    source_id: str, uri: str, file_sha256: str, page_count: int, mtime_ns: int
) -> list[Chunk]:
    """One `pdf_page` (image) chunk per page, identified by the file's hash and the page number."""
    return [
        Chunk(
            source_id=source_id,
            uri=uri,
            kind=ChunkKind.PDF_PAGE,
            locator=locator_json(page=page),
            content_hash=sha256_hex(f"{file_sha256}:{page}"),
            mtime_ns=mtime_ns,
            text="",  # the page's words are already searchable through its pdf_text chunk
        )
        for page in range(1, page_count + 1)
    ]
```

- [ ] **Step 4: Implement `chunking/image.py`**

```python
"""Image files as one image chunk each, plus the loader that prepares pixels for the model."""

from __future__ import annotations

import io

from PIL import Image, ImageOps, UnidentifiedImageError

from agi_memory.chunking.common import ChunkingError
from agi_memory.types import Chunk, ChunkKind


def load_image(data: bytes, max_side: int) -> Image.Image:
    """Decode, apply EXIF orientation, convert to RGB and thumbnail to `max_side`."""
    try:
        with Image.open(io.BytesIO(data)) as opened:
            image = ImageOps.exif_transpose(opened).convert("RGB")
    except (UnidentifiedImageError, OSError, Image.DecompressionBombError) as error:
        raise ChunkingError(f"unreadable image: {error}") from error
    image.thumbnail((max_side, max_side))
    return image


def image_chunk(source_id: str, uri: str, file_sha256: str, mtime_ns: int) -> Chunk:
    """The single chunk for an image file. Its text is the path, so file names are searchable."""
    return Chunk(
        source_id=source_id,
        uri=uri,
        kind=ChunkKind.IMAGE,
        locator="{}",
        content_hash=file_sha256,
        mtime_ns=mtime_ns,
        text=uri,
    )
```

- [ ] **Step 5: Update the spec**

In `docs/superpowers/specs/2026-10-12-memory-engine-design.md` section 5, the `chunks` description says "`text` ... `''` for images" is not stated there, but section 6's Chunking line ends with "Image: thumbnailed to 1024 px, kind `image`." Replace that sentence with: "Image: thumbnailed to 1024 px (EXIF orientation applied), kind `image`, chunk text = the file's relative path so file names are keyword-searchable; `pdf_page` chunks have empty text (their words live in the page's `pdf_text` chunk)." (Use the Edit tool; match the existing sentence exactly.)

- [ ] **Step 6: Run the gate**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass (5 pdf + 7 image test cases). If pdfium's `PdfiumError` is not raised for the corrupt PDF in your pypdfium2 version, widen the `except` in `PdfPages.__init__` to `(pdfium.PdfiumError, ValueError)` and ledger the ruling.

- [ ] **Step 7: Commit**

```bash
git add sidecar/src sidecar/tests docs
git commit -m "feat(chunking): PDF pages as text and images, and image loading"
```

### Task 5: Store schema v2 with sources and files

**Files:**
- Modify: `sidecar/src/agi_memory/types.py`, `sidecar/src/agi_memory/store/schema.py`, `sidecar/src/agi_memory/store/flat.py`, `sidecar/tests/helpers.py`
- Create: `sidecar/src/agi_memory/store/catalog.py`
- Test: `sidecar/tests/test_catalog.py`

**Interfaces:**
- Consumes: `FlatStore.upsert/delete` (phase 2), `Chunk`.
- Produces `types.FileRecord(source_id: str, uri: str, size: int, mtime_ns: int, sha256: str)` frozen dataclass.
- Produces `SCHEMA_VERSION = 2` and tables `sources(id PRIMARY KEY, root, last_scan_ns)` and `files(source_id, uri, size, mtime_ns, sha256, PRIMARY KEY (source_id, uri))`. A v1 index is refused with `IndexMismatchError` ("reindex required").
- Produces `store.catalog.Catalog(db: sqlite3.Connection)`: SQL-only helpers used by `FlatStore` (no locking of its own): `register_source`, `mark_scanned`, `file_records`, `write_file`, `chunk_ids_for_uri`, `chunk_ids_for_uris`, `chunk_ids_for_source`, `delete_file_rows`, `delete_source_rows`.
- Produces on `FlatStore` (all hold the store lock): `register_source(source_id: str, root: str) -> None`, `mark_scanned(source_id: str, scanned_ns: int) -> None`, `file_records(source_id: str) -> dict[str, FileRecord]`, `replace_file(record: FileRecord, chunks: Sequence[Chunk], vectors: np.ndarray) -> None` (every chunk must carry `record.source_id`/`record.uri`, else `ValueError`; upserts the new chunks, deletes that file's chunks that are not in the new set, then writes the file row last so a crash leaves the file stale and it is re-indexed), `touch_file(record: FileRecord) -> None` (file row only), `remove_files(source_id: str, uris: Iterable[str]) -> int` (files removed), `delete_source(source_id: str) -> int` (chunks removed).
- Produces helpers in `tests/helpers.py`: `make_file_chunks(uri, count, *, source_id="s", tag="v1") -> list[Chunk]` and `file_record(uri, *, source_id="s", sha="sha1", size=1, mtime_ns=1) -> FileRecord`.

- [ ] **Step 1: Test helpers and failing tests**

Append to `sidecar/tests/helpers.py` (add `FileRecord` to the existing `agi_memory.types` import):

```python
def make_file_chunks(uri: str, count: int, *, source_id: str = "s", tag: str = "v1") -> list[Chunk]:
    """`count` chunks of one file; changing `tag` changes every chunk's content (and id)."""
    return [
        Chunk(
            source_id=source_id,
            uri=uri,
            kind=ChunkKind.TEXT,
            locator=f'{{"i":{i}}}',
            content_hash=f"{uri}-{tag}-{i}",
            mtime_ns=1,
            text=f"{uri} {tag} part {i}",
        )
        for i in range(count)
    ]


def file_record(
    uri: str, *, source_id: str = "s", sha: str = "sha1", size: int = 1, mtime_ns: int = 1
) -> FileRecord:
    return FileRecord(source_id=source_id, uri=uri, size=size, mtime_ns=mtime_ns, sha256=sha)
```

`sidecar/tests/test_catalog.py`:

```python
import sqlite3

import numpy as np
import pytest
from helpers import DIM, file_record, make_file_chunks, unit_vectors

from agi_memory.store import FlatStore, IndexMismatchError


def put(store, uri, count, *, tag="v1", sha="sha1", source_id="s", mtime_ns=1):
    chunks = make_file_chunks(uri, count, source_id=source_id, tag=tag)
    store.replace_file(
        file_record(uri, source_id=source_id, sha=sha, mtime_ns=mtime_ns), chunks, unit_vectors(count)
    )
    return chunks


def test_a_new_source_has_no_files(store):
    store.register_source("s", "C:/data")
    assert store.file_records("s") == {}


def test_register_source_is_idempotent(store):
    store.register_source("s", "C:/a")
    store.register_source("s", "C:/b")
    store.mark_scanned("s", 123)  # must not raise for a registered source


def test_replace_file_stores_chunks_and_the_file_record(store):
    chunks = put(store, "a.txt", 3)
    assert store.count() == 3
    assert set(store.get([c.id for c in chunks])) == {c.id for c in chunks}
    assert store.file_records("s") == {"a.txt": file_record("a.txt")}


def test_replacing_with_new_content_removes_the_stale_chunks(store):
    old = put(store, "a.txt", 5, tag="v1", sha="one")
    new = put(store, "a.txt", 2, tag="v2", sha="two", mtime_ns=9)
    assert store.count() == 2
    assert store.get([c.id for c in old]) == {}
    assert set(store.get([c.id for c in new])) == {c.id for c in new}
    assert store.file_records("s")["a.txt"].sha256 == "two"
    assert store.file_records("s")["a.txt"].mtime_ns == 9


def test_replacing_with_identical_content_changes_nothing(store):
    put(store, "a.txt", 3)
    put(store, "a.txt", 3)
    assert store.count() == 3


def test_a_file_that_becomes_empty_keeps_its_record_but_no_chunks(store):
    put(store, "a.txt", 3)
    store.replace_file(file_record("a.txt", sha="empty"), [], np.zeros((0, DIM), dtype=np.float32))
    assert store.count() == 0
    assert store.file_records("s")["a.txt"].sha256 == "empty"


def test_chunks_of_another_file_or_source_are_rejected_and_nothing_is_written(store):
    wrong_uri = make_file_chunks("b.txt", 1)
    wrong_source = make_file_chunks("a.txt", 1, source_id="other")
    for chunks in (wrong_uri, wrong_source):
        with pytest.raises(ValueError, match="belong"):
            store.replace_file(file_record("a.txt"), chunks, unit_vectors(1))
    assert store.count() == 0
    assert store.file_records("s") == {}


def test_touch_file_updates_only_the_record(store):
    chunks = put(store, "a.txt", 2)
    store.touch_file(file_record("a.txt", mtime_ns=99))
    assert store.file_records("s")["a.txt"].mtime_ns == 99
    assert set(store.get([c.id for c in chunks])) == {c.id for c in chunks}


def test_remove_files_deletes_chunks_and_records(store):
    put(store, "a.txt", 2)
    keep = put(store, "b.txt", 3)
    assert store.remove_files("s", ["a.txt", "never-existed.txt"]) == 1
    assert store.count() == 3
    assert set(store.file_records("s")) == {"b.txt"}
    assert set(store.get([c.id for c in keep])) == {c.id for c in keep}


def test_delete_source_only_touches_that_source(store):
    put(store, "a.txt", 2, source_id="one")
    keep = put(store, "a.txt", 2, source_id="two")
    store.register_source("one", "C:/one")
    assert store.delete_source("one") == 2
    assert store.file_records("one") == {}
    assert store.count() == 2
    assert set(store.get([c.id for c in keep])) == {c.id for c in keep}


def test_the_same_path_in_two_sources_does_not_collide(store):
    a = put(store, "same.txt", 1, source_id="one")
    b = put(store, "same.txt", 1, source_id="two")
    assert a[0].id != b[0].id
    assert store.count() == 2


def test_stale_chunks_are_not_searchable_after_replace(store):
    old = put(store, "a.txt", 4, tag="v1")
    vectors = unit_vectors(1, seed=3)
    new = make_file_chunks("a.txt", 1, tag="v2")
    store.replace_file(file_record("a.txt", sha="two"), new, vectors)
    hits = store.search_vectors(vectors[0], 10)
    assert [h.id for h in hits] == [new[0].id]
    assert not {h.id for h in hits} & {c.id for c in old}


def test_an_index_from_the_previous_schema_is_refused(tmp_path):
    directory = tmp_path / "idx"
    FlatStore(directory, model_id="fake", dim=DIM).close()
    db = sqlite3.connect(directory / "meta.db")
    db.execute("UPDATE meta SET value = '1' WHERE key = 'schema_version'")
    db.commit()
    db.close()
    with pytest.raises(IndexMismatchError, match="reindex"):
        FlatStore(directory, model_id="fake", dim=DIM)
```

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_catalog.py -q`
Expected: FAIL (`ImportError: cannot import name 'FileRecord'` from `agi_memory.types` via `helpers`).

- [ ] **Step 3: Add `FileRecord` to `types.py`**

Append to `sidecar/src/agi_memory/types.py`:

```python
@dataclass(frozen=True, slots=True)
class FileRecord:
    """What the index remembers about one indexed file, to detect changes cheaply."""

    source_id: str
    uri: str
    size: int
    mtime_ns: int
    sha256: str
```

- [ ] **Step 4: Schema v2**

In `sidecar/src/agi_memory/store/schema.py` set `SCHEMA_VERSION = 2`, and insert these statements inside the `SCHEMA` string immediately after the `CREATE INDEX IF NOT EXISTS chunks_source ...` line:

```sql
CREATE INDEX IF NOT EXISTS chunks_source_uri ON chunks(source_id, uri);

CREATE TABLE IF NOT EXISTS sources (
    id           TEXT PRIMARY KEY,
    root         TEXT NOT NULL,
    last_scan_ns INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
    source_id TEXT NOT NULL,
    uri       TEXT NOT NULL,
    size      INTEGER NOT NULL,
    mtime_ns  INTEGER NOT NULL,
    sha256    TEXT NOT NULL,
    PRIMARY KEY (source_id, uri)
);
```

Update the module docstring's first line to mention "schema version 2: index identity, chunk metadata, the keyword index and the source/file catalog".

- [ ] **Step 5: Implement `store/catalog.py`**

```python
"""SQL for the `sources` and `files` tables (change detection) and per-file chunk lookups.

Pure SQL helpers over an open connection. They take no locks and never commit on their own
schedule beyond the explicit `with db:` blocks: `FlatStore` owns locking and ordering.
"""

from __future__ import annotations

import sqlite3
from collections.abc import Sequence

from agi_memory.types import FileRecord


class Catalog:
    def __init__(self, db: sqlite3.Connection) -> None:
        self._db = db

    def register_source(self, source_id: str, root: str) -> None:
        with self._db:
            self._db.execute(
                "INSERT INTO sources (id, root, last_scan_ns) VALUES (?, ?, 0) "
                "ON CONFLICT(id) DO UPDATE SET root = excluded.root",
                (source_id, root),
            )

    def mark_scanned(self, source_id: str, scanned_ns: int) -> None:
        with self._db:
            self._db.execute(
                "UPDATE sources SET last_scan_ns = ? WHERE id = ?", (scanned_ns, source_id)
            )

    def file_records(self, source_id: str) -> dict[str, FileRecord]:
        rows = self._db.execute(
            "SELECT uri, size, mtime_ns, sha256 FROM files WHERE source_id = ?", (source_id,)
        )
        return {
            uri: FileRecord(source_id, uri, size, mtime_ns, sha256)
            for uri, size, mtime_ns, sha256 in rows
        }

    def write_file(self, record: FileRecord) -> None:
        with self._db:
            self._db.execute(
                "INSERT INTO files (source_id, uri, size, mtime_ns, sha256) VALUES (?, ?, ?, ?, ?) "
                "ON CONFLICT(source_id, uri) DO UPDATE SET "
                "size = excluded.size, mtime_ns = excluded.mtime_ns, sha256 = excluded.sha256",
                (record.source_id, record.uri, record.size, record.mtime_ns, record.sha256),
            )

    def chunk_ids_for_uri(self, source_id: str, uri: str) -> list[str]:
        rows = self._db.execute(
            "SELECT id FROM chunks WHERE source_id = ? AND uri = ?", (source_id, uri)
        )
        return [chunk_id for (chunk_id,) in rows]

    def chunk_ids_for_uris(self, source_id: str, uris: Sequence[str]) -> list[str]:
        marks = ",".join("?" * len(uris))
        rows = self._db.execute(
            f"SELECT id FROM chunks WHERE source_id = ? AND uri IN ({marks})", [source_id, *uris]
        )
        return [chunk_id for (chunk_id,) in rows]

    def chunk_ids_for_source(self, source_id: str) -> list[str]:
        rows = self._db.execute("SELECT id FROM chunks WHERE source_id = ?", (source_id,))
        return [chunk_id for (chunk_id,) in rows]

    def delete_file_rows(self, source_id: str, uris: Sequence[str]) -> int:
        marks = ",".join("?" * len(uris))
        with self._db:
            cursor = self._db.execute(
                f"DELETE FROM files WHERE source_id = ? AND uri IN ({marks})", [source_id, *uris]
            )
        return cursor.rowcount

    def delete_source_rows(self, source_id: str) -> None:
        with self._db:
            self._db.execute("DELETE FROM files WHERE source_id = ?", (source_id,))
            self._db.execute("DELETE FROM sources WHERE id = ?", (source_id,))
```

- [ ] **Step 6: Wire it into `FlatStore`**

In `sidecar/src/agi_memory/store/flat.py`:

1. Add imports: `from agi_memory.store.catalog import Catalog` and extend the types import to `from agi_memory.types import Chunk, ChunkKind, FileRecord, Filters, VectorHit`.
2. In `__init__`, right after `self._lock = threading.RLock()` and `self._db = sqlite3.connect(...)`, add `self._catalog = Catalog(self._db)`.
3. Add these methods to the class, before `_validate_query`:

```python
    # --- sources and files: change detection for incremental indexing ------------------

    def register_source(self, source_id: str, root: str) -> None:
        with self._lock:
            self._catalog.register_source(source_id, root)

    def mark_scanned(self, source_id: str, scanned_ns: int) -> None:
        with self._lock:
            self._catalog.mark_scanned(source_id, scanned_ns)

    def file_records(self, source_id: str) -> dict[str, FileRecord]:
        with self._lock:
            return self._catalog.file_records(source_id)

    def replace_file(self, record: FileRecord, chunks: Sequence[Chunk], vectors: np.ndarray) -> None:
        """Make the index hold exactly `chunks` for this file, then remember the file.

        The file row is written last: a crash in between leaves the file looking stale, so the
        next sync re-indexes it (chunk ids are content-derived, so redoing it is harmless).
        """
        if any(c.source_id != record.source_id or c.uri != record.uri for c in chunks):
            raise ValueError(f"every chunk must belong to {record.source_id}:{record.uri}")
        with self._lock:
            self.upsert(chunks, vectors)
            keep = {chunk.id for chunk in chunks}
            stale = [
                chunk_id
                for chunk_id in self._catalog.chunk_ids_for_uri(record.source_id, record.uri)
                if chunk_id not in keep
            ]
            self.delete(stale)
            self._catalog.write_file(record)

    def touch_file(self, record: FileRecord) -> None:
        """Update a file's remembered size/mtime/hash when its content is known to be unchanged."""
        with self._lock:
            self._catalog.write_file(record)

    def remove_files(self, source_id: str, uris: Iterable[str]) -> int:
        """Forget files (and their chunks); returns how many file records were removed."""
        removed = 0
        with self._lock:
            for batch in _batches(list(uris)):
                self.delete(self._catalog.chunk_ids_for_uris(source_id, batch))
                removed += self._catalog.delete_file_rows(source_id, batch)
        return removed

    def delete_source(self, source_id: str) -> int:
        """Forget a whole source; returns how many chunks were removed."""
        with self._lock:
            ids = self._catalog.chunk_ids_for_source(source_id)
            self.delete(ids)
            self._catalog.delete_source_rows(source_id)
            return len(ids)
```

- [ ] **Step 7: Run the gate**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass (13 new tests). The existing store tests still pass (the schema bump only affects fresh indexes).

- [ ] **Step 8: Commit**

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(store): sources and files catalog with atomic per-file replace"
```

### Task 6: Indexer (incremental sync)

**Files:**
- Create: `sidecar/src/agi_memory/indexer.py`
- Test: `sidecar/tests/test_indexer.py`

**Interfaces:**
- Consumes: `scan`/`ScannedFile`/`ScanResult` (Task 2), `chunk_text_document`/`NotTextError` (Task 3), `PdfPages`/`pdf_text_chunks`/`pdf_page_chunks`/`load_image`/`image_chunk` (Task 4), `FlatStore` catalog methods (Task 5), `Embedder`, `Settings`, `Route`, `FileRecord`, `ChunkingError`.
- Produces `agi_memory.indexer`:
  - `FileError(uri: str, message: str)`; `SyncReport` with ints `added, updated, unchanged, removed, denied, ignored, unsupported, too_large, chunks_written` and `errors: list[FileError]`.
  - `Indexer(store: FlatStore, embedder: Embedder, settings: Settings, *, clock: Callable[[], int] = time.time_ns)`.
  - `Indexer.sync(source_id: str, root: Path, *, on_progress: Callable[[int, int], None] | None = None) -> SyncReport`: raises `FileNotFoundError` if `root` is not a directory; otherwise brings the store in line with the folder (see behaviour below). Per-file read/chunk failures are recorded in `errors` (binary files count as `unsupported`); embedder and store failures propagate.
- Behaviour: unchanged `(size, mtime_ns)` -> no work; changed -> read, SHA-256, if equal to the stored hash only `touch_file`; text chunks are batched across files (flush at `flush_chunks` pending chunks); PDFs and images are embedded per file (page images in batches of `image_batch_size`, rendered lazily); files that disappeared are removed unless they sit under an unreadable directory prefix.

- [ ] **Step 1: Write the failing tests**

`sidecar/tests/test_indexer.py`:

```python
import builtins
import io
import os

import pytest
from pdf_fixture import make_text_pdf
from PIL import Image

from agi_memory.config import Settings
from agi_memory.embedding import FakeEmbedder
from agi_memory.indexer import Indexer
from agi_memory.store import FlatStore
from agi_memory.types import ChunkKind

DIM = 16


class CountingEmbedder(FakeEmbedder):
    def __init__(self) -> None:
        super().__init__(DIM)
        self.document_calls = 0
        self.image_calls = 0

    def embed_documents(self, texts):
        self.document_calls += 1
        return super().embed_documents(texts)

    def embed_images(self, images):
        self.image_calls += 1
        return super().embed_images(images)


@pytest.fixture
def embedder():
    return CountingEmbedder()


@pytest.fixture
def settings(tmp_path):
    return Settings(index_dir=tmp_path / "idx")


@pytest.fixture
def store(tmp_path):
    flat = FlatStore(tmp_path / "idx", model_id="fake", dim=DIM)
    yield flat
    flat.close()


@pytest.fixture
def indexer(store, embedder, settings):
    return Indexer(store, embedder, settings, clock=lambda: 42)


@pytest.fixture
def root(tmp_path):
    folder = tmp_path / "src"
    folder.mkdir()
    return folder


def write(root, relative, content="x"):
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return path


def all_chunks(store, source_id="s"):
    return store.get(store._catalog.chunk_ids_for_source(source_id))


def test_first_sync_indexes_everything_and_reports_it(indexer, store, root):
    write(root, "a.js", "const alpha = 1")
    write(root, "docs/b.md", "# Title\nbeta text")
    report = indexer.sync("s", root)
    assert (report.added, report.updated, report.unchanged, report.removed) == (2, 0, 0, 0)
    assert report.chunks_written == store.count() == 2
    assert set(store.file_records("s")) == {"a.js", "docs/b.md"}


def test_searching_finds_what_was_indexed(indexer, store, embedder, root):
    write(root, "a.js", "function frobnicate() {}")
    indexer.sync("s", root)
    (hit,) = store.search_vectors(embedder.embed_query("frobnicate"), 1)
    assert store.get([hit.id])[hit.id].uri == "a.js"


def test_an_unchanged_folder_costs_no_embedding_work(indexer, store, embedder, root):
    write(root, "a.js", "one")
    write(root, "b.md", "two")
    indexer.sync("s", root)
    before = {i: c for i, c in all_chunks(store).items()}
    calls = embedder.document_calls
    report = indexer.sync("s", root)
    assert (report.added, report.updated, report.unchanged) == (0, 0, 2)
    assert embedder.document_calls == calls
    assert all_chunks(store) == before


def test_a_modified_file_is_reindexed_and_stale_chunks_disappear(indexer, store, root):
    path = write(root, "a.txt", "\n".join(f"line {i}" for i in range(100)))
    indexer.sync("s", root)
    assert store.count() == 3
    path.write_text("short now", encoding="utf-8")
    report = indexer.sync("s", root)
    assert (report.updated, report.added) == (1, 0)
    assert store.count() == 1
    assert "short now" in next(iter(all_chunks(store).values())).text


def test_rewriting_identical_bytes_only_updates_the_record(indexer, store, embedder, root):
    path = write(root, "a.txt", "same content")
    indexer.sync("s", root)
    calls = embedder.document_calls
    os.utime(path, ns=(10**18, 10**18))  # new mtime, same bytes
    report = indexer.sync("s", root)
    assert (report.unchanged, report.updated) == (1, 0)
    assert embedder.document_calls == calls
    assert store.file_records("s")["a.txt"].mtime_ns == 10**18


def test_deleted_files_are_removed_from_the_index(indexer, store, root):
    write(root, "keep.txt", "keep")
    gone = write(root, "gone.txt", "gone")
    indexer.sync("s", root)
    gone.unlink()
    report = indexer.sync("s", root)
    assert report.removed == 1
    assert set(store.file_records("s")) == {"keep.txt"}
    assert store.count() == 1


def test_secret_files_are_never_opened_or_indexed(indexer, store, root, monkeypatch):
    write(root, ".env", "API_KEY=TOPSECRET-123")
    write(root, "Backend/.env.local", "DB_PASSWORD=TOPSECRET-456")
    write(root, "keys/server.pem", "-----BEGIN PRIVATE KEY-----")
    write(root, "ok.js", "const fine = true")
    opened: list[str] = []
    real_open = builtins.open

    def spy(file, *args, **kwargs):
        opened.append(str(file))
        return real_open(file, *args, **kwargs)

    monkeypatch.setattr(builtins, "open", spy)
    monkeypatch.setattr(io, "open", spy)
    report = indexer.sync("s", root)
    assert report.denied == 3
    assert not [name for name in opened if name.endswith((".env", ".env.local", ".pem"))]
    assert all("TOPSECRET" not in c.text for c in all_chunks(store).values())
    assert set(store.file_records("s")) == {"ok.js"}


def test_binary_files_with_code_extensions_are_skipped_not_errors(indexer, root):
    (root / "weird.js").write_bytes(b"\x00\x01\x02 not really javascript")
    write(root, "fine.js", "ok")
    report = indexer.sync("s", root)
    assert (report.unsupported, report.added, report.errors) == (1, 1, [])


def test_a_file_that_cannot_be_read_is_an_error_and_the_rest_is_indexed(
    indexer, store, root, monkeypatch
):
    write(root, "locked.txt", "cannot read me")
    write(root, "fine.txt", "readable")
    real = type(root).read_bytes

    def flaky(self):
        if self.name == "locked.txt":
            raise PermissionError("file is locked")
        return real(self)

    monkeypatch.setattr(type(root), "read_bytes", flaky)
    report = indexer.sync("s", root)
    assert [(e.uri, "locked" in e.message) for e in report.errors] == [("locked.txt", True)]
    assert report.added == 1
    assert set(store.file_records("s")) == {"fine.txt"}


def test_a_corrupt_pdf_and_a_bad_image_are_recorded_and_others_survive(indexer, root):
    (root / "broken.pdf").write_bytes(b"%PDF-1.4 not really")
    (root / "broken.png").write_bytes(b"not an image")
    write(root, "fine.txt", "readable")
    report = indexer.sync("s", root)
    assert sorted(e.uri for e in report.errors) == ["broken.pdf", "broken.png"]
    assert report.added == 1


def test_a_text_pdf_yields_text_and_page_image_chunks(indexer, store, root):
    (root / "paper.pdf").write_bytes(make_text_pdf(["Confidence intervals", "z table"]))
    report = indexer.sync("s", root)
    kinds = sorted(c.kind for c in all_chunks(store).values())
    assert kinds == [ChunkKind.PDF_PAGE] * 2 + [ChunkKind.PDF_TEXT] * 2
    assert (report.added, report.chunks_written) == (1, 4)


def test_an_image_file_is_one_image_chunk(indexer, store, root):
    buffer = io.BytesIO()
    Image.new("RGB", (64, 48), "red").save(buffer, format="PNG")
    (root / "pics").mkdir()
    (root / "pics" / "red square.png").write_bytes(buffer.getvalue())
    indexer.sync("s", root)
    (chunk,) = all_chunks(store).values()
    assert (chunk.kind, chunk.uri, chunk.text) == (
        ChunkKind.IMAGE, "pics/red square.png", "pics/red square.png",
    )  # fmt: skip


def test_text_chunks_are_embedded_in_batches_across_files(store, embedder, settings, root):
    for number in range(30):
        write(root, f"f{number}.txt", f"file number {number}")
    batched = Indexer(store, embedder, Settings(index_dir=settings.index_dir, flush_chunks=10))
    batched.sync("s", root)
    assert store.count() == 30
    assert embedder.document_calls <= 4  # 30 chunks in batches of ~10, not one call per file


def test_an_unreadable_directory_never_deletes_its_indexed_files(
    indexer, store, root, monkeypatch
):
    write(root, "ok.txt", "fine")
    write(root, "locked/hidden.txt", "indexed earlier")
    indexer.sync("s", root)
    real_scandir = os.scandir

    def flaky(path):
        if str(path).endswith("locked"):
            raise PermissionError("access denied")
        return real_scandir(path)

    monkeypatch.setattr(os, "scandir", flaky)
    report = indexer.sync("s", root)
    assert report.removed == 0
    assert set(store.file_records("s")) == {"ok.txt", "locked/hidden.txt"}
    assert [e.uri for e in report.errors] == ["locked/"]


def test_a_missing_source_root_is_an_error(indexer, tmp_path):
    with pytest.raises(FileNotFoundError, match="source root"):
        indexer.sync("s", tmp_path / "unplugged-drive")


def test_two_sources_with_identical_files_stay_separate(indexer, store, root, tmp_path):
    second = tmp_path / "other"
    second.mkdir()
    write(root, "same.txt", "identical")
    write(second, "same.txt", "identical")
    indexer.sync("one", root)
    indexer.sync("two", second)
    assert store.count() == 2
    assert indexer.sync("one", root).unchanged == 1


def test_progress_is_reported_per_file_and_scan_time_is_recorded(indexer, store, root):
    write(root, "a.txt", "a")
    write(root, "b.txt", "b")
    seen = []
    indexer.sync("s", root, on_progress=lambda done, total: seen.append((done, total)))
    assert seen == [(1, 2), (2, 2)]
    row = store._db.execute("SELECT last_scan_ns FROM sources WHERE id = 's'").fetchone()
    assert row == (42,)


def test_empty_and_hostile_files_do_not_break_a_sync(indexer, store, root):
    write(root, "empty.txt", "")
    (root / "invalid.txt").write_bytes(b"ok \xff\xfe bad")
    write(root, "huge-line.js", "x" * 1_000_000)
    write(root, "crlf.md", "# H\r\nbody\r\n")
    report = indexer.sync("s", root)
    assert report.errors == []
    assert report.added == 4  # the empty file is "indexed" with zero chunks
    assert store.count() == 3
```

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_indexer.py -q`
Expected: FAIL (`ModuleNotFoundError: agi_memory.indexer`).

- [ ] **Step 3: Implement `indexer.py`**

```python
"""Indexer: brings a store in line with a folder, doing embedding work only for what changed.

Flow for one `sync`: scan the folder, diff against the files the store already knows, remove what
disappeared, then for every new or changed file read it once, hash it and chunk it. Text chunks
from many small files are embedded together (one GPU call has a ~200 ms fixed cost, so one call
per file would dominate indexing time); PDFs and images are embedded per file because they are
heavy and rendered lazily. Each file lands in the store atomically (`FlatStore.replace_file`).

Failure policy: a file that cannot be read or chunked is recorded in the report and skipped; an
embedder or store failure aborts the sync (everything committed so far stays valid, and the
next sync continues from there because file records are written last).
"""

from __future__ import annotations

import hashlib
import time
from collections.abc import Callable
from dataclasses import dataclass, field
from pathlib import Path

import numpy as np

from agi_memory.chunking.common import ChunkingError
from agi_memory.chunking.image import image_chunk, load_image
from agi_memory.chunking.pdf import PdfPages, pdf_page_chunks, pdf_text_chunks
from agi_memory.chunking.text import NotTextError, chunk_text_document
from agi_memory.config import Settings
from agi_memory.embedding.base import Embedder
from agi_memory.sources.folder import ScannedFile, scan
from agi_memory.sources.rules import Route
from agi_memory.store import FlatStore
from agi_memory.types import Chunk, FileRecord

_TEXT_ROUTES = frozenset({Route.CODE, Route.MARKDOWN, Route.TEXT})


@dataclass(frozen=True, slots=True)
class FileError:
    uri: str
    message: str


@dataclass(slots=True)
class SyncReport:
    added: int = 0
    updated: int = 0
    unchanged: int = 0
    removed: int = 0
    denied: int = 0
    ignored: int = 0
    unsupported: int = 0
    too_large: int = 0
    chunks_written: int = 0
    errors: list[FileError] = field(default_factory=list)


@dataclass(slots=True)
class _Pending:
    record: FileRecord
    chunks: list[Chunk]
    is_new: bool


class Indexer:
    """Keeps one store in sync with folders, one `sync` call per source."""

    def __init__(
        self,
        store: FlatStore,
        embedder: Embedder,
        settings: Settings,
        *,
        clock: Callable[[], int] = time.time_ns,
    ) -> None:
        self._store = store
        self._embedder = embedder
        self._settings = settings
        self._clock = clock

    def sync(
        self,
        source_id: str,
        root: Path,
        *,
        on_progress: Callable[[int, int], None] | None = None,
    ) -> SyncReport:
        """Index new and changed files under `root`, drop vanished ones, return what happened."""
        if not root.is_dir():
            raise FileNotFoundError(f"source root is not a folder: {root}")
        scanned = scan(root, self._settings)
        report = SyncReport(
            denied=scanned.denied,
            ignored=scanned.ignored,
            unsupported=scanned.unsupported,
            too_large=scanned.too_large,
        )
        report.errors.extend(FileError(uri, message) for uri, message in scanned.unreadable)
        self._store.register_source(source_id, str(root))
        known = self._store.file_records(source_id)

        on_disk = {file.uri for file in scanned.files}
        protected = tuple(prefix for prefix, _ in scanned.unreadable)  # could not look in there
        gone = [u for u in sorted(known) if u not in on_disk and not u.startswith(protected)]
        report.removed = self._store.remove_files(source_id, gone)

        pending: list[_Pending] = []
        total = len(scanned.files)
        for number, file in enumerate(scanned.files, start=1):
            try:
                self._index_file(source_id, file, known.get(file.uri), report, pending)
            except NotTextError:
                report.unsupported += 1
            except (ChunkingError, OSError) as error:
                report.errors.append(FileError(file.uri, str(error)))
            if sum(len(item.chunks) for item in pending) >= self._settings.flush_chunks:
                self._flush(pending, report)
            if on_progress is not None:
                on_progress(number, total)
        self._flush(pending, report)
        self._store.mark_scanned(source_id, self._clock())
        return report

    # --- per file -----------------------------------------------------------------------

    def _index_file(
        self,
        source_id: str,
        file: ScannedFile,
        known: FileRecord | None,
        report: SyncReport,
        pending: list[_Pending],
    ) -> None:
        if known is not None and known.size == file.size and known.mtime_ns == file.mtime_ns:
            report.unchanged += 1
            return
        data = file.path.read_bytes()
        digest = hashlib.sha256(data).hexdigest()
        record = FileRecord(source_id, file.uri, file.size, file.mtime_ns, digest)
        if known is not None and known.sha256 == digest:
            self._store.touch_file(record)  # same bytes, new mtime: nothing to embed
            report.unchanged += 1
            return
        is_new = known is None
        if file.route in _TEXT_ROUTES:
            chunks = chunk_text_document(
                source_id, file.uri, file.route, data, file.mtime_ns, self._settings
            )
            pending.append(_Pending(record, chunks, is_new))
        elif file.route is Route.PDF:
            self._index_pdf(record, data, file.mtime_ns, is_new, report)
        else:
            self._index_image(record, data, file.mtime_ns, is_new, report)

    def _flush(self, pending: list[_Pending], report: SyncReport) -> None:
        """Embed every pending text chunk in one call, then store each file atomically."""
        if not pending:
            return
        texts = [chunk.text for item in pending for chunk in item.chunks]
        vectors = (
            self._embedder.embed_documents(texts)
            if texts
            else np.zeros((0, self._embedder.dim), dtype=np.float32)
        )
        offset = 0
        for item in pending:
            count = len(item.chunks)
            self._commit(item.record, item.chunks, vectors[offset : offset + count], item.is_new, report)
            offset += count
        pending.clear()

    def _commit(
        self,
        record: FileRecord,
        chunks: list[Chunk],
        vectors: np.ndarray,
        is_new: bool,
        report: SyncReport,
    ) -> None:
        self._store.replace_file(record, chunks, vectors)
        report.chunks_written += len(chunks)
        if is_new:
            report.added += 1
        else:
            report.updated += 1

    def _index_pdf(
        self, record: FileRecord, data: bytes, mtime_ns: int, is_new: bool, report: SyncReport
    ) -> None:
        with PdfPages(data) as pdf:
            text_chunks = pdf_text_chunks(record.source_id, record.uri, pdf, mtime_ns, self._settings)
            page_chunks = pdf_page_chunks(
                record.source_id, record.uri, record.sha256, len(pdf), mtime_ns
            )
            text_vectors = self._embedder.embed_documents([c.text for c in text_chunks])
            page_vectors = np.zeros((len(page_chunks), self._embedder.dim), dtype=np.float32)
            step = self._settings.image_batch_size
            for start in range(0, len(page_chunks), step):
                stop = min(start + step, len(page_chunks))
                images = [pdf.render(i, self._settings.pdf_render_scale) for i in range(start, stop)]
                page_vectors[start:stop] = self._embedder.embed_images(images)
        self._commit(
            record,
            text_chunks + page_chunks,
            np.concatenate([text_vectors, page_vectors]),
            is_new,
            report,
        )

    def _index_image(
        self, record: FileRecord, data: bytes, mtime_ns: int, is_new: bool, report: SyncReport
    ) -> None:
        image = load_image(data, self._settings.image_max_side)
        chunk = image_chunk(record.source_id, record.uri, record.sha256, mtime_ns)
        self._commit(record, [chunk], self._embedder.embed_images([image]), is_new, report)
```

- [ ] **Step 4: Run the tests, then the gate**

Run: `uv run pytest tests/test_indexer.py -v`
Expected: 18 passed. If the secret-file spy test fails because pathlib opens files through another entry point, find that entry point (print `opened`) and patch it too; the assertion that no denied path is opened must stay.
Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(indexer): incremental folder sync with cross-file embedding batches"
```

### Task 7: Sample-data integration test and phase 3 gate

**Files:**
- Test: `sidecar/tests/test_sample_data.py`
- Modify: `docs/ARCHITECTURE.md`, `docs/STATUS.md`

**Interfaces:** Consumes `Indexer`, `FlatStore`, `FakeEmbedder`. Produces a test that skips when `Sample Data/` is absent.

- [ ] **Step 1: Write the test**

`sidecar/tests/test_sample_data.py`:

```python
import builtins
import io
from collections import Counter
from pathlib import Path

import pytest

from agi_memory.config import Settings
from agi_memory.embedding import FakeEmbedder
from agi_memory.indexer import Indexer
from agi_memory.store import FlatStore
from agi_memory.types import ChunkKind

SAMPLE = Path(__file__).resolve().parents[2] / "Sample Data"
pytestmark = pytest.mark.skipif(not SAMPLE.is_dir(), reason="Sample Data/ is not present")


@pytest.fixture
def indexed(tmp_path, monkeypatch):
    embedder = FakeEmbedder(16)
    store = FlatStore(tmp_path / "idx", model_id=embedder.model_id, dim=16)
    opened: list[str] = []
    real_open = builtins.open

    def spy(file, *args, **kwargs):
        opened.append(str(file))
        return real_open(file, *args, **kwargs)

    monkeypatch.setattr(builtins, "open", spy)
    monkeypatch.setattr(io, "open", spy)
    indexer = Indexer(store, embedder, Settings(index_dir=tmp_path / "idx"))
    report = indexer.sync("sample", SAMPLE)
    yield store, indexer, report, opened
    store.close()


def test_the_real_sample_folder_is_indexed_and_secrets_are_not(indexed):
    store, _, report, opened = indexed
    uris = set(store.file_records("sample"))
    assert not [u for u in uris if ".env" in u or "node_modules" in u or ".git/" in u]
    assert not [name for name in opened if ".env" in Path(name).name]
    assert report.denied >= 4  # Backend/.env, Frontend/.env, .envsample, lockfiles, .git, node_modules
    assert report.errors == []
    for expected in (
        "NextPlore/Backend/src/utils/jwt.js",
        "NextPlore/README.md",
        "NextPlore/Documentation/Images/dataschemas.png",
        "Week 10 Prep.pdf",
        "Week 11 Prep.pdf",
    ):
        assert expected in uris


def test_chunk_kinds_match_what_is_on_disk(indexed):
    store, _, _, _ = indexed
    ids = store._catalog.chunk_ids_for_source("sample")
    kinds = Counter(chunk.kind for chunk in store.get(ids).values())
    assert kinds[ChunkKind.PDF_PAGE] == 20  # 9 + 11 pages
    assert kinds[ChunkKind.IMAGE] == 5  # 3 PNGs in the repo + 2 pictures at the top level
    assert kinds[ChunkKind.CODE] > 100
    assert kinds[ChunkKind.MARKDOWN] >= 3


def test_syncing_again_does_no_work(indexed):
    store, indexer, first, _ = indexed
    count = store.count()
    second = indexer.sync("sample", SAMPLE)
    assert (second.added, second.updated) == (0, 0)
    assert second.unchanged == first.added
    assert store.count() == count
```

- [ ] **Step 2: Run it**

Run: `uv run pytest tests/test_sample_data.py -v`
Expected: 3 passed (or 3 skipped if `Sample Data/` is missing). If a count assertion is off, print the actual `Counter` and decide whether the rule or the number is wrong; do not loosen a secret-related assertion.

- [ ] **Step 3: Phase 3 docs gate**

In `docs/ARCHITECTURE.md`: replace the module-table row `| \`chunking/\`, \`sources/\`, \`jobs.py\` | Turn files into chunks and index them incrementally | phase 3 |` with these four rows:

```
| `sources/` | `rules` (secret deny list, file routing), `folder` (gitignore-aware scanner, never follows links) | built |
| `chunking/` | Line-window and Markdown-section chunks, PDF pages (text + image), images | built |
| `indexer.py` | `Indexer.sync`: scan, diff against the files table, chunk, embed, store; embeds only what changed | built |
| `jobs.py` | Background job wrapper with progress for the API | phase 5 |
```

Add under "On-disk format" a paragraph: "Schema version 2 adds `sources(id, root, last_scan_ns)` and `files(source_id, uri, size, mtime_ns, sha256)`. `FlatStore.replace_file` upserts a file's new chunks, deletes its stale ones, and writes the `files` row last, so an interrupted sync leaves the file stale and it is redone. A v1 index is refused (`reindex required`); there is no v1 data to migrate." Add to "Invariants": "6. **Sync safety.** Denied files are never opened; links are never followed; a directory that cannot be read never causes its indexed files to be deleted; a missing source root is an error." In `docs/STATUS.md` set phase 3 to `done` and update "Next" to "Phase 4 tasks in `superpowers/plans/2026-10-12-memory-engine-phases-3-4.md`".

- [ ] **Step 4: Gate and commit**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass.

```bash
git add sidecar/tests docs
git commit -m "test(indexer): index the real sample folder; docs: phase 3 status"
```

---

## Phase 4: Search and evaluation

### Task 8: Keyword search in the store

**Files:**
- Create: `sidecar/src/agi_memory/store/keyword.py`, `sidecar/src/agi_memory/store/filters.py`
- Modify: `sidecar/src/agi_memory/types.py`, `sidecar/src/agi_memory/store/flat.py`
- Test: `sidecar/tests/test_keyword.py`

**Interfaces:**
- Consumes: FTS5 table `chunks_fts` (phase 2), `Filters`, `split_identifiers`.
- Produces `types.KeywordHit(id: str, score: float)` frozen dataclass (higher = better; score is `-bm25`).
- Produces `store.keyword.build_match(query: str) -> str`: the FTS5 MATCH expression, `""` when the query has no usable word. Words come from `\w+` tokens of the lower-cased query plus the words inside camelCase/snake_case identifiers (`split_identifiers`), minus one-character words and a fixed English stoplist (`the a an of to in for and or is are how what do does with on at by from that this`), de-duplicated in order, capped at 32, each wrapped in double quotes and joined with ` OR `.
- Produces `store.filters.filter_clause(filters: Filters) -> tuple[str, list[object]] | None`: SQL `WHERE` fragment (columns `kind`, `source_id`, `uri`, `mtime_ns`) and parameters, `None` when `filters` has no restriction; an empty `kinds` set yields the fragment `0` (matches nothing).
- Produces `FlatStore.keyword_search(query: str, n: int, filters: Filters | None = None) -> list[KeywordHit]`: bm25 over `text` (weight 1.0) and `identifiers` (weight 0.5), ordered by (score descending, id); `[]` for an empty match expression or `n <= 0`; `_filter_mask` now reuses `filter_clause`.

- [ ] **Step 1: Write the failing tests**

`sidecar/tests/test_keyword.py`:

```python
import pytest
from helpers import make_chunk, unit_vectors

from agi_memory.store.keyword import build_match
from agi_memory.types import ChunkKind, Filters


@pytest.mark.parametrize(
    ("query", "expected"),
    [
        ("How do I refresh the access token?", '"refresh" OR "access" OR "token"'),
        ("addCountries", '"addcountries" OR "add" OR "countries"'),
        ("add_countries seed", '"add_countries" OR "seed" OR "add" OR "countries"'),
        ("the of and", ""),
        ("", ""),
        ("   ", ""),
        ("!!! ??? ***", ""),
        ('say "hi" * NEAR( x )', '"say" OR "hi" OR "near"'),
        ("token token TOKEN", '"token"'),
        ("a b c", ""),
    ],
)
def test_build_match(query, expected):
    assert build_match(query) == expected


def test_build_match_caps_the_number_of_words():
    query = " ".join(f"word{i}" for i in range(100))
    assert build_match(query).count(" OR ") == 31


def fill(store):
    chunks = [
        make_chunk("a", "Backend/addcountries.js\nconst addCountries = seed countries csv"),
        make_chunk("b", "Backend/jwt.js\nrefresh token generate access token", kind=ChunkKind.CODE),
        make_chunk("c", "notes.md\nthe quick brown fox refresh the token", source_id="docs"),
    ]
    store.upsert(chunks, unit_vectors(3))
    return {c.uri: c.id for c in chunks}


def test_ranking_prefers_more_relevant_chunks(store):
    ids = fill(store)
    hits = store.keyword_search("refresh access token", 5)
    assert [h.id for h in hits][0] == ids["b.txt"]
    assert {h.id for h in hits} == {ids["b.txt"], ids["c.txt"]}
    assert hits[0].score > hits[1].score > 0


def test_identifiers_are_findable_by_their_words_and_whole(store):
    ids = fill(store)
    assert store.keyword_search("countries", 5)[0].id == ids["a.txt"]
    assert store.keyword_search("addcountries", 5)[0].id == ids["a.txt"]


def test_file_names_in_the_text_are_searchable(store):
    ids = fill(store)
    assert store.keyword_search("jwt.js", 5)[0].id == ids["b.txt"]


def test_filters_apply_to_keyword_search(store):
    ids = fill(store)
    only_docs = store.keyword_search("token", 5, Filters(source_id="docs"))
    assert [h.id for h in only_docs] == [ids["c.txt"]]
    only_code = store.keyword_search("token", 5, Filters(kinds=frozenset({ChunkKind.CODE})))
    assert [h.id for h in only_code] == [ids["b.txt"]]
    assert store.keyword_search("token", 5, Filters(kinds=frozenset())) == []


def test_deleted_and_replaced_chunks_leave_the_keyword_index(store):
    ids = fill(store)
    store.delete([ids["b.txt"]])
    assert [h.id for h in store.keyword_search("token", 5)] == [ids["c.txt"]]


@pytest.mark.parametrize(
    "query", ["", "   ", "the of and", "!!!", 'say "hi" * NEAR( x ) OR AND NOT', "x" * 5000]
)
def test_odd_queries_never_raise(store, query):
    fill(store)
    assert isinstance(store.keyword_search(query, 5), list)


def test_empty_results_and_limits(store):
    fill(store)
    assert store.keyword_search("nonexistentword", 5) == []
    assert store.keyword_search("token", 0) == []
    assert len(store.keyword_search("token", 1)) == 1


def test_ties_are_broken_by_id(store):
    chunks = [make_chunk(name, "identical words here") for name in ("zulu", "alpha", "mike")]
    store.upsert(chunks, unit_vectors(3))
    hits = store.keyword_search("identical words", 5)
    assert [h.id for h in hits] == sorted(c.id for c in chunks)
```

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_keyword.py -q`
Expected: FAIL (`ModuleNotFoundError: agi_memory.store.keyword`).

- [ ] **Step 3: Add `KeywordHit` to `types.py`**

Append:

```python
@dataclass(frozen=True, slots=True)
class KeywordHit:
    """One result of a keyword (FTS5/bm25) search."""

    id: str
    score: float  # negated bm25, so higher is better like the vector scores
```

- [ ] **Step 4: Implement `store/keyword.py`**

```python
"""Turns free text into a safe FTS5 MATCH expression.

Every word is double-quoted, so user text can never be parsed as FTS5 syntax (NEAR, OR, ^, *,
column filters...). Words are OR-ed: bm25 ranks chunks matching more, rarer words first.
"""

from __future__ import annotations

import re

from agi_memory.store.identifiers import split_identifiers

_WORD = re.compile(r"\w+")
_STOPWORDS = frozenset(
    "the a an of to in for and or is are how what do does with on at by from that this".split()
)
_MAX_WORDS = 32


def build_match(query: str) -> str:
    """FTS5 MATCH expression for `query`, or `""` if it has no searchable word."""
    words = _WORD.findall(query.lower()) + _WORD.findall(split_identifiers(query))
    kept = [w for w in dict.fromkeys(words) if len(w) > 1 and w not in _STOPWORDS]
    return " OR ".join(f'"{word}"' for word in kept[:_MAX_WORDS])
```

- [ ] **Step 5: Implement `store/filters.py` and wire both into `FlatStore`**

`sidecar/src/agi_memory/store/filters.py`:

```python
"""SQL for `Filters`, shared by vector search (row masks) and keyword search (joins)."""

from __future__ import annotations

from agi_memory.types import Filters


def filter_clause(filters: Filters) -> tuple[str, list[object]] | None:
    """A WHERE fragment over the `chunks` columns and its parameters; None if unrestricted."""
    clauses: list[str] = []
    params: list[object] = []
    if filters.kinds is not None:
        if not filters.kinds:
            return "0", []  # an empty set of allowed kinds matches nothing
        clauses.append(f"kind IN ({','.join('?' * len(filters.kinds))})")
        params.extend(sorted(kind.value for kind in filters.kinds))
    if filters.source_id is not None:
        clauses.append("source_id = ?")
        params.append(filters.source_id)
    if filters.uri_prefix is not None:
        clauses.append("substr(uri, 1, ?) = ?")  # substr, not LIKE: '%' and '_' are literal
        params.extend([len(filters.uri_prefix), filters.uri_prefix])
    if filters.modified_after_ns is not None:
        clauses.append("mtime_ns > ?")
        params.append(filters.modified_after_ns)
    if not clauses:
        return None
    return " AND ".join(clauses), params
```

In `sidecar/src/agi_memory/store/flat.py`:

1. Imports: add `from agi_memory.store.filters import filter_clause`, `from agi_memory.store.keyword import build_match`, and change the types import to include `KeywordHit`.
2. Replace the whole body of `_filter_mask` with:

```python
    def _filter_mask(self, filters: Filters, size: int) -> np.ndarray:
        """Boolean mask over rows [0, size) for chunks that satisfy `filters`."""
        clause = filter_clause(filters)
        if clause is None:
            return np.ones(size, dtype=bool)
        where, params = clause
        mask = np.zeros(size, dtype=bool)
        rows = [row for (row,) in self._db.execute(f"SELECT vec_row FROM chunks WHERE {where}", params)]
        mask[np.asarray(rows, dtype=np.intp)] = True
        return mask
```

3. Add this method before `close()`:

```python
    def keyword_search(
        self, query: str, n: int, filters: Filters | None = None
    ) -> list[KeywordHit]:
        """Top-`n` chunks by bm25 over text and identifiers, ordered by (-score, id)."""
        match = build_match(query)
        if not match or n <= 0:
            return []
        sql = (
            "SELECT c.id, bm25(chunks_fts, 1.0, 0.5) AS score FROM chunks_fts "
            "JOIN chunks c ON c.rid = chunks_fts.rowid WHERE chunks_fts MATCH ?"
        )
        params: list[object] = [match]
        clause = filter_clause(filters) if filters is not None else None
        if clause is not None:
            sql += f" AND ({clause[0]})"
            params.extend(clause[1])
        sql += " ORDER BY score, c.id LIMIT ?"
        params.append(n)
        with self._lock:
            rows = self._db.execute(sql, params).fetchall()
        return [KeywordHit(id=chunk_id, score=-score) for chunk_id, score in rows]
```

- [ ] **Step 6: Run the gate**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass (24 new test cases; the earlier filter tests in `test_flat_search.py` still pass through the shared clause). If `test_ties_are_broken_by_id` fails because bm25 scores differ in the last float digit for identical text, sort in Python after fetching (`rows.sort(key=lambda r: (round(r[1], 9), r[0]))`), ledger the ruling, and keep the test.

- [ ] **Step 7: Commit**

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(store): FTS5 keyword search with safe query building and shared filters"
```

### Task 9: Rank fusion and the Searcher

**Files:**
- Create: `sidecar/src/agi_memory/search/__init__.py`, `fusion.py`, `searcher.py`
- Modify: `sidecar/src/agi_memory/types.py`
- Test: `sidecar/tests/test_fusion.py`, `sidecar/tests/test_searcher.py`

**Interfaces:**
- Produces `types.SearchHit(id, uri, kind, locator, score, vector_score, keyword_rank, snippet)` frozen dataclass: `score: float` (fused), `vector_score: float | None` (cosine, `None` if the vector side did not return it), `keyword_rank: int | None` (1-based, `None` if the keyword side did not return it), `snippet: str` (the chunk text after its first line, whitespace collapsed, at most 240 characters; `""` for images and PDF page images).
- Produces `search.fusion.reciprocal_rank_fusion(rankings: Sequence[Sequence[str]], k: int = 60) -> list[tuple[str, float]]`: score `sum(1 / (k + rank))` over the lists an id appears in (an id repeated inside one list counts once, at its best rank), sorted by (-score, id).
- Produces `search.searcher.Searcher(store: FlatStore, embedder: Embedder, *, pool: int = 200, rrf_k: int = 60)` with `search(query: str, k: int = 10, filters: Filters | None = None) -> list[SearchHit]`; returns `[]` for a blank query or `k <= 0`.

- [ ] **Step 1: Write the failing tests**

`sidecar/tests/test_fusion.py`:

```python
import pytest

from agi_memory.search.fusion import reciprocal_rank_fusion


def test_an_item_ranked_well_by_both_lists_wins():
    fused = reciprocal_rank_fusion([["a", "b", "c"], ["c", "b", "d"]])
    assert [i for i, _ in fused][:2] == ["b", "c"]  # b: ranks 2+2, c: ranks 3+1, a only once


def test_scores_follow_the_formula():
    ((first, score),) = reciprocal_rank_fusion([["x"], ["x"]], k=60)
    assert first == "x"
    assert score == pytest.approx(2 / 61)


def test_ties_are_broken_by_id():
    fused = reciprocal_rank_fusion([["b"], ["a"]])
    assert [i for i, _ in fused] == ["a", "b"]  # identical scores, id order


def test_a_single_list_keeps_its_order():
    assert [i for i, _ in reciprocal_rank_fusion([["x", "y", "z"]])] == ["x", "y", "z"]


def test_empty_inputs():
    assert reciprocal_rank_fusion([]) == []
    assert reciprocal_rank_fusion([[], []]) == []


def test_a_repeated_id_counts_once_at_its_best_rank():
    fused = dict(reciprocal_rank_fusion([["a", "a", "b"]], k=60))
    assert fused["a"] == pytest.approx(1 / 61)
    assert fused["b"] == pytest.approx(1 / 63)
```

`sidecar/tests/test_searcher.py`:

```python
import pytest

from agi_memory.config import Settings
from agi_memory.embedding import FakeEmbedder
from agi_memory.indexer import Indexer
from agi_memory.search.searcher import Searcher
from agi_memory.store import FlatStore
from agi_memory.types import ChunkKind, Filters

DIM = 64


@pytest.fixture
def world(tmp_path):
    embedder = FakeEmbedder(DIM)
    store = FlatStore(tmp_path / "idx", model_id=embedder.model_id, dim=DIM)
    root = tmp_path / "src"
    (root / "Backend").mkdir(parents=True)
    (root / "docs").mkdir()
    (root / "Backend" / "jwt.js").write_text("function signRefreshToken() { refresh token jwt }")
    (root / "Backend" / "addcountries.js").write_text("async function addCountries() { seed }")
    (root / "docs" / "notes.md").write_text("# Notes\nthe quick brown fox jumps over the lazy dog")
    Indexer(store, embedder, Settings(index_dir=tmp_path / "idx")).sync("repo", root)
    yield Searcher(store, embedder), store
    store.close()


def test_a_semantic_style_query_finds_the_right_file(world):
    searcher, _ = world
    hits = searcher.search("how do I refresh the jwt token", k=3)
    assert hits[0].uri == "Backend/jwt.js"


def test_an_exact_identifier_is_found_through_the_keyword_side(world):
    searcher, _ = world
    top = searcher.search("addCountries", k=3)[0]
    assert top.uri == "Backend/addcountries.js"
    assert top.keyword_rank == 1


def test_hits_carry_the_documented_fields(world):
    searcher, _ = world
    hit = searcher.search("quick brown fox", k=1)[0]
    assert (hit.uri, hit.kind) == ("docs/notes.md", ChunkKind.MARKDOWN)
    assert hit.locator.startswith('{"lines":[')
    assert hit.score > 0
    assert hit.vector_score is not None
    assert hit.keyword_rank is not None
    assert "quick brown fox" in hit.snippet
    assert not hit.snippet.startswith("docs/notes.md")  # the path line is dropped
    assert len(hit.snippet) <= 240


def test_filters_reach_both_searches(world):
    searcher, _ = world
    hits = searcher.search("token", k=5, filters=Filters(uri_prefix="docs/"))
    assert hits
    assert all(h.uri.startswith("docs/") for h in hits)


def test_k_limits_the_results(world):
    searcher, _ = world
    assert len(searcher.search("token fox countries", k=2)) == 2


@pytest.mark.parametrize("query", ["", "   ", "the of and", "!!! ???", 'say "hi" * NEAR( x )'])
def test_blank_and_odd_queries_do_not_raise(world, query):
    searcher, _ = world
    assert isinstance(searcher.search(query), list)


def test_a_five_thousand_character_query_works(world):
    searcher, _ = world
    assert isinstance(searcher.search("token " * 1000), list)


def test_non_positive_k_returns_nothing(world):
    searcher, _ = world
    assert searcher.search("token", k=0) == []


def test_results_are_deterministic(world):
    searcher, _ = world
    first = [h.id for h in searcher.search("token fox countries", k=5)]
    again = [h.id for h in searcher.search("token fox countries", k=5)]
    assert first == again


def test_a_chunk_deleted_between_ranking_and_lookup_is_skipped(world, monkeypatch):
    searcher, store = world
    real_get = store.get
    monkeypatch.setattr(store, "get", lambda ids: {k: v for k, v in real_get(ids).items() if k != ids[0]})
    hits = searcher.search("refresh jwt token", k=3)
    assert len(hits) <= 2  # the top-ranked chunk vanished and is silently dropped
```

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_fusion.py tests/test_searcher.py -q`
Expected: FAIL (`ModuleNotFoundError: agi_memory.search`).

- [ ] **Step 3: Add `SearchHit` to `types.py`**

Append:

```python
@dataclass(frozen=True, slots=True)
class SearchHit:
    """One fused search result, with enough detail to show it and to debug the ranking."""

    id: str
    uri: str
    kind: ChunkKind
    locator: str
    score: float  # reciprocal-rank-fusion score (relative, not a probability)
    vector_score: float | None  # cosine similarity; None if the vector search did not return it
    keyword_rank: int | None  # 1-based bm25 rank; None if the keyword search did not return it
    snippet: str
```

- [ ] **Step 4: Implement fusion and the searcher**

`sidecar/src/agi_memory/search/__init__.py`:

```python
"""Search: vector + keyword retrieval fused into one ranking."""
```

`sidecar/src/agi_memory/search/fusion.py`:

```python
"""Reciprocal rank fusion: merge ranked lists without comparing their incompatible scores."""

from __future__ import annotations

from collections.abc import Sequence


def reciprocal_rank_fusion(
    rankings: Sequence[Sequence[str]], k: int = 60
) -> list[tuple[str, float]]:
    """`score(id) = sum(1 / (k + rank))` over every list containing `id`, best first.

    Ties are broken by id so the order never depends on dictionary or insertion order.
    """
    scores: dict[str, float] = {}
    for ranking in rankings:
        seen: set[str] = set()
        for rank, chunk_id in enumerate(ranking, start=1):
            if chunk_id in seen:
                continue
            seen.add(chunk_id)
            scores[chunk_id] = scores.get(chunk_id, 0.0) + 1.0 / (k + rank)
    return sorted(scores.items(), key=lambda item: (-item[1], item[0]))
```

`sidecar/src/agi_memory/search/searcher.py`:

```python
"""Searcher: embeds the query, runs vector and keyword search, fuses and describes the hits."""

from __future__ import annotations

from agi_memory.embedding.base import Embedder
from agi_memory.search.fusion import reciprocal_rank_fusion
from agi_memory.store import FlatStore
from agi_memory.types import Chunk, Filters, SearchHit

_SNIPPET_CHARS = 240


def _snippet(chunk: Chunk) -> str:
    """The chunk body (text after the path line), whitespace collapsed and shortened."""
    _, _, body = chunk.text.partition("\n")
    return " ".join(body.split())[:_SNIPPET_CHARS]


class Searcher:
    """Hybrid search over one store. Query embedding costs ~200 ms; the rest is milliseconds."""

    def __init__(
        self, store: FlatStore, embedder: Embedder, *, pool: int = 200, rrf_k: int = 60
    ) -> None:
        self._store = store
        self._embedder = embedder
        self._pool = pool
        self._rrf_k = rrf_k

    def search(self, query: str, k: int = 10, filters: Filters | None = None) -> list[SearchHit]:
        """Top-`k` chunks for `query`, best first; `[]` for a blank query or `k <= 0`."""
        if not query.strip() or k <= 0:
            return []
        vector_hits = self._store.search_vectors(
            self._embedder.embed_query(query), self._pool, filters
        )
        keyword_hits = self._store.keyword_search(query, self._pool, filters)
        fused = reciprocal_rank_fusion(
            [[h.id for h in vector_hits], [h.id for h in keyword_hits]], self._rrf_k
        )[:k]
        vector_scores = {h.id: h.score for h in vector_hits}
        keyword_ranks = {h.id: rank for rank, h in enumerate(keyword_hits, start=1)}
        chunks = self._store.get([chunk_id for chunk_id, _ in fused])
        hits = []
        for chunk_id, score in fused:
            chunk = chunks.get(chunk_id)
            if chunk is None:  # deleted between ranking and lookup
                continue
            hits.append(
                SearchHit(
                    id=chunk_id,
                    uri=chunk.uri,
                    kind=chunk.kind,
                    locator=chunk.locator,
                    score=score,
                    vector_score=vector_scores.get(chunk_id),
                    keyword_rank=keyword_ranks.get(chunk_id),
                    snippet=_snippet(chunk),
                )
            )
        return hits
```

- [ ] **Step 5: Run the gate**

Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass (6 fusion + 14 searcher test cases). If `test_a_semantic_style_query_finds_the_right_file` fails, print the hits: the fake embedder is a bag of words, so this test relies on "refresh", "token" and "jwt" overlapping `jwt.js`'s text; fix the corpus text if it is genuinely ambiguous, not the ranking.

- [ ] **Step 6: Commit**

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(search): reciprocal rank fusion and the hybrid Searcher"
```

### Task 10: Evaluation harness

**Files:**
- Create: `sidecar/src/agi_memory/eval/__init__.py`, `golden.py`, `metrics.py`, `runner.py`, `__main__.py`
- Test: `sidecar/tests/test_eval.py`

**Interfaces:**
- Consumes: `Searcher`, `Indexer`, `FlatStore`, `Embedder`, `Settings`, `SyncReport`.
- Produces `eval.golden`: `GoldenQuery(query: str, expect: tuple[str, ...], forbid: tuple[str, ...], note: str)` and `load_golden(path: Path) -> list[GoldenQuery]` (YAML list of mappings with keys `query`, `expect`, optional `forbid`, `note`; raises `ValueError` for non-list files, missing/empty `query` or `expect`, unknown keys, non-string items, duplicate queries).
- Produces `eval.metrics`: `first_relevant_rank(uris: Sequence[str], expect: Sequence[str]) -> int | None` (1-based rank of the first uri containing any `expect` substring); `QueryResult(query, first_rank: int | None, forbidden: tuple[str, ...], top_uris: tuple[str, ...])`; `Metrics(n, top1, recall3, recall10, mrr, forbidden_hits)` frozen dataclass; `summarize(results: Sequence[QueryResult]) -> Metrics` (`top1` = share with rank 1, `recall3`/`recall10` = share with rank <= 3 / <= 10, `mrr` = mean of 1/rank with 0 for misses, `forbidden_hits` = total forbidden matches).
- Produces `eval.runner`: `build_index(store, embedder, settings, data_dir: Path, source_id: str = "sample") -> SyncReport`; `evaluate(golden, searcher, k: int = 10) -> list[QueryResult]` (a forbidden string counts when it appears in any top-k uri); `baseline_dict(metrics, embedder_id) -> dict[str, object]`; `compare_to_baseline(metrics, baseline, tolerance: float = 0.02) -> list[str]` (a message per metric among `top1, recall3, recall10, mrr` that fell more than `tolerance`, plus one when `forbidden_hits > 0`); `render_report(label, embedder_id, today, metrics, results, sync, golden) -> str` (Markdown); `run(argv: Sequence[str], make_embedder: Callable[[bool], Embedder], today: str) -> int`.
- CLI: `uv run python -m agi_memory.eval [--data PATH] [--golden PATH] [--report-dir PATH] [--label NAME] [--cpu] [--write-baseline]`; defaults `--data "../Sample Data"`, `--golden eval/golden.yaml`, `--report-dir ../docs/eval`, `--label baseline`; writes `<report-dir>/<today>-<label>.md`; with `--write-baseline` writes `<report-dir>/baseline.json`; otherwise compares to it when present; exit status 1 on any regression or forbidden hit, else 0. BLAS threads are pinned to 1 during the run.

- [ ] **Step 1: Write the failing tests**

`sidecar/tests/test_eval.py`:

```python
import json

import pytest

from agi_memory.embedding import FakeEmbedder
from agi_memory.eval.golden import GoldenQuery, load_golden
from agi_memory.eval.metrics import Metrics, QueryResult, first_relevant_rank, summarize
from agi_memory.eval.runner import baseline_dict, compare_to_baseline, render_report, run


def write_golden(path, text):
    path.write_text(text, encoding="utf-8")
    return path


def test_golden_file_loads_with_defaults(tmp_path):
    path = write_golden(
        tmp_path / "g.yaml",
        "- query: find jwt\n  expect: [jwt.js]\n- query: secrets\n  expect: [README]\n"
        "  forbid: ['.env']\n  note: privacy\n",
    )
    first, second = load_golden(path)
    assert first == GoldenQuery("find jwt", ("jwt.js",), (), "")
    assert second == GoldenQuery("secrets", ("README",), (".env",), "privacy")


@pytest.mark.parametrize(
    "text",
    [
        "just: a mapping\n",
        "- expect: [a]\n",
        "- query: ''\n  expect: [a]\n",
        "- query: q\n",
        "- query: q\n  expect: []\n",
        "- query: q\n  expect: [a]\n  surprise: 1\n",
        "- query: q\n  expect: [1]\n",
        "- query: q\n  expect: [a]\n- query: q\n  expect: [b]\n",
    ],
)
def test_malformed_golden_files_are_rejected(tmp_path, text):
    with pytest.raises(ValueError, match="golden"):
        load_golden(write_golden(tmp_path / "g.yaml", text))


def test_first_relevant_rank():
    uris = ["a/x.js", "b/jwt.js", "README.md"]
    assert first_relevant_rank(uris, ["jwt"]) == 2
    assert first_relevant_rank(uris, ["README", "x.js"]) == 1
    assert first_relevant_rank(uris, ["missing"]) is None
    assert first_relevant_rank([], ["a"]) is None


def test_summary_metrics():
    results = [
        QueryResult("q1", 1, (), ()),
        QueryResult("q2", 2, (), ()),
        QueryResult("q3", 5, (), ()),
        QueryResult("q4", None, (".env",), ()),
    ]
    m = summarize(results)
    assert (m.n, m.top1, m.recall3, m.recall10) == (4, 0.25, 0.5, 0.75)
    assert m.mrr == pytest.approx((1 + 1 / 2 + 1 / 5 + 0) / 4)
    assert m.forbidden_hits == 1


def test_comparing_to_the_baseline():
    baseline = {"top1": 0.8, "recall3": 0.9, "recall10": 1.0, "mrr": 0.85}
    same = Metrics(10, 0.8, 0.9, 1.0, 0.85, 0)
    assert compare_to_baseline(same, baseline) == []
    within_tolerance = Metrics(10, 0.79, 0.9, 1.0, 0.85, 0)
    assert compare_to_baseline(within_tolerance, baseline) == []
    worse = Metrics(10, 0.7, 0.9, 1.0, 0.85, 0)
    (message,) = compare_to_baseline(worse, baseline)
    assert "top1" in message
    leaked = Metrics(10, 0.8, 0.9, 1.0, 0.85, 2)
    assert any("forbid" in m for m in compare_to_baseline(leaked, baseline))


def test_baseline_dict_has_the_compared_keys():
    data = baseline_dict(Metrics(3, 0.5, 0.6, 0.7, 0.8, 0), "fake")
    assert data == {"embedder": "fake", "n": 3, "top1": 0.5, "recall3": 0.6, "recall10": 0.7, "mrr": 0.8}


@pytest.fixture
def corpus(tmp_path):
    data = tmp_path / "data"
    (data / "Backend").mkdir(parents=True)
    (data / "docs").mkdir()
    (data / "Backend" / "jwt.js").write_text("function refreshToken() { jwt refresh token }")
    (data / "Backend" / "addcountries.js").write_text("async function addCountries() { seed csv }")
    (data / "docs" / "README.md").write_text("# Project\nsetup the backend environment variables")
    (data / ".env").write_text("JWT_SECRET=hunter2")
    golden = write_golden(
        tmp_path / "golden.yaml",
        "- query: refresh the jwt token\n  expect: [jwt.js]\n"
        "- query: addCountries\n  expect: [addcountries.js]\n"
        "- query: environment variables\n  expect: [README]\n  forbid: ['.env']\n",
    )
    return data, golden


def make_fake(cpu):
    return FakeEmbedder(32)


def test_the_cli_runs_end_to_end_writes_a_report_and_a_baseline(tmp_path, corpus):
    data, golden = corpus
    reports = tmp_path / "reports"
    args = ["--data", str(data), "--golden", str(golden), "--report-dir", str(reports), "--write-baseline"]
    assert run(args, make_fake, "2026-10-12") == 0
    report = (reports / "2026-10-12-baseline.md").read_text(encoding="utf-8")
    assert "refresh the jwt token" in report
    assert "top1" in report
    baseline = json.loads((reports / "baseline.json").read_text(encoding="utf-8"))
    assert baseline["embedder"] == "fake-hashed-bow-v1"
    assert baseline["n"] == 3
    assert baseline["top1"] == 1.0  # the fake embedder plus keyword search ranks these first


def test_the_cli_fails_when_quality_drops_below_the_baseline(tmp_path, corpus):
    data, golden = corpus
    reports = tmp_path / "reports"
    reports.mkdir()
    (reports / "baseline.json").write_text(
        json.dumps({"embedder": "x", "n": 3, "top1": 1.0, "recall3": 1.0, "recall10": 1.0, "mrr": 1.0}),
        encoding="utf-8",
    )
    impossible = write_golden(
        tmp_path / "bad.yaml",
        "- query: refresh the jwt token\n  expect: [jwt.js]\n"
        "- query: something that is not there\n  expect: [no-such-file]\n",
    )
    args = ["--data", str(data), "--golden", str(impossible), "--report-dir", str(reports)]
    assert run(args, make_fake, "2026-10-12") == 1


def test_the_cli_fails_when_a_forbidden_file_shows_up(tmp_path, corpus):
    data, _ = corpus
    reports = tmp_path / "reports"
    leaky = write_golden(
        tmp_path / "leaky.yaml", "- query: environment variables\n  expect: [README]\n  forbid: [README]\n"
    )
    args = ["--data", str(data), "--golden", str(leaky), "--report-dir", str(reports)]
    assert run(args, make_fake, "2026-10-12") == 1


def test_report_lists_every_query_and_misses(tmp_path):
    metrics = Metrics(2, 0.5, 0.5, 0.5, 0.5, 0)
    results = [QueryResult("found one", 1, (), ("a.js",)), QueryResult("lost one", None, (), ("z.js",))]
    golden = [GoldenQuery("found one", ("a",), (), ""), GoldenQuery("lost one", ("b",), (), "")]
    from agi_memory.indexer import SyncReport

    text = render_report("t", "fake", "2026-10-12", metrics, results, SyncReport(added=2), golden)
    assert "found one" in text and "lost one" in text
    assert "miss" in text.lower()
```

- [ ] **Step 2: Run to see them fail**

Run: `uv run pytest tests/test_eval.py -q`
Expected: FAIL (`ModuleNotFoundError: agi_memory.eval`).

- [ ] **Step 3: Implement `eval/golden.py` and `eval/metrics.py`**

`sidecar/src/agi_memory/eval/__init__.py`:

```python
"""Retrieval-quality evaluation: golden queries, metrics, a runner and a CLI."""
```

`sidecar/src/agi_memory/eval/golden.py`:

```python
"""The golden query set: queries with known-good answers, kept in a reviewable YAML file."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

import yaml

_KEYS = {"query", "expect", "forbid", "note"}


@dataclass(frozen=True, slots=True)
class GoldenQuery:
    query: str
    expect: tuple[str, ...]  # a hit is relevant if its uri contains any of these
    forbid: tuple[str, ...]  # no top-k hit's uri may contain any of these (privacy checks)
    note: str


def _strings(value: object, where: str) -> tuple[str, ...]:
    if value is None:
        return ()
    if not isinstance(value, list) or not all(isinstance(item, str) and item for item in value):
        raise ValueError(f"golden {where} must be a list of non-empty strings")
    return tuple(value)


def load_golden(path: Path) -> list[GoldenQuery]:
    """Parse and validate a golden file; a malformed file is an error, never a silent skip."""
    raw = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(raw, list):
        raise ValueError(f"golden file {path} must be a list of queries")
    queries: list[GoldenQuery] = []
    seen: set[str] = set()
    for number, entry in enumerate(raw, start=1):
        where = f"entry {number}"
        if not isinstance(entry, dict):
            raise ValueError(f"golden {where} must be a mapping")
        unknown = set(entry) - _KEYS
        if unknown:
            raise ValueError(f"golden {where} has unknown keys {sorted(unknown)}")
        query = entry.get("query")
        if not isinstance(query, str) or not query.strip():
            raise ValueError(f"golden {where} needs a non-empty query")
        expect = _strings(entry.get("expect"), f"{where} expect")
        if not expect:
            raise ValueError(f"golden {where} needs at least one expect string")
        if query in seen:
            raise ValueError(f"golden query repeated: {query!r}")
        seen.add(query)
        note = entry.get("note", "")
        queries.append(
            GoldenQuery(query, expect, _strings(entry.get("forbid"), f"{where} forbid"), str(note))
        )
    return queries
```

`sidecar/src/agi_memory/eval/metrics.py`:

```python
"""Retrieval metrics over golden-query results."""

from __future__ import annotations

from collections.abc import Sequence
from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class QueryResult:
    query: str
    first_rank: int | None  # 1-based rank of the first relevant hit within the top-k
    forbidden: tuple[str, ...]  # forbidden strings found in top-k uris
    top_uris: tuple[str, ...]


@dataclass(frozen=True, slots=True)
class Metrics:
    n: int
    top1: float
    recall3: float
    recall10: float
    mrr: float
    forbidden_hits: int


def first_relevant_rank(uris: Sequence[str], expect: Sequence[str]) -> int | None:
    """1-based position of the first uri containing any `expect` substring, else None."""
    for rank, uri in enumerate(uris, start=1):
        if any(fragment in uri for fragment in expect):
            return rank
    return None


def summarize(results: Sequence[QueryResult]) -> Metrics:
    n = len(results)
    if n == 0:
        return Metrics(0, 0.0, 0.0, 0.0, 0.0, 0)
    ranks = [r.first_rank for r in results]
    return Metrics(
        n=n,
        top1=sum(1 for r in ranks if r == 1) / n,
        recall3=sum(1 for r in ranks if r is not None and r <= 3) / n,
        recall10=sum(1 for r in ranks if r is not None and r <= 10) / n,
        mrr=sum(1 / r for r in ranks if r is not None) / n,
        forbidden_hits=sum(len(r.forbidden) for r in results),
    )
```

- [ ] **Step 4: Implement `eval/runner.py` and `eval/__main__.py`**

`sidecar/src/agi_memory/eval/runner.py`:

```python
"""Runs the golden queries against a real index and reports, compares and gates on quality."""

from __future__ import annotations

import argparse
import json
import tempfile
from collections.abc import Callable, Sequence
from pathlib import Path

from threadpoolctl import threadpool_limits

from agi_memory.config import Settings
from agi_memory.embedding.base import Embedder
from agi_memory.eval.golden import GoldenQuery, load_golden
from agi_memory.eval.metrics import Metrics, QueryResult, first_relevant_rank, summarize
from agi_memory.indexer import Indexer, SyncReport
from agi_memory.search.searcher import Searcher
from agi_memory.store import FlatStore

_COMPARED = ("top1", "recall3", "recall10", "mrr")


def build_index(
    store: FlatStore, embedder: Embedder, settings: Settings, data_dir: Path, source_id: str = "sample"
) -> SyncReport:
    """Index `data_dir` into `store` exactly as production would."""
    return Indexer(store, embedder, settings).sync(source_id, data_dir)


def evaluate(golden: Sequence[GoldenQuery], searcher: Searcher, k: int = 10) -> list[QueryResult]:
    results = []
    for item in golden:
        uris = tuple(hit.uri for hit in searcher.search(item.query, k))
        forbidden = tuple(sorted({f for f in item.forbid for uri in uris if f in uri}))
        results.append(QueryResult(item.query, first_relevant_rank(uris, item.expect), forbidden, uris))
    return results


def baseline_dict(metrics: Metrics, embedder_id: str) -> dict[str, object]:
    return {
        "embedder": embedder_id,
        "n": metrics.n,
        "top1": metrics.top1,
        "recall3": metrics.recall3,
        "recall10": metrics.recall10,
        "mrr": metrics.mrr,
    }


def compare_to_baseline(
    metrics: Metrics, baseline: dict[str, object], tolerance: float = 0.02
) -> list[str]:
    """One message per metric that fell more than `tolerance`, plus one for any forbidden hit."""
    problems = []
    for name in _COMPARED:
        old = float(baseline[name])  # type: ignore[arg-type]
        new = getattr(metrics, name)
        if new < old - tolerance:
            problems.append(f"{name} fell from {old:.3f} to {new:.3f}")
    if metrics.forbidden_hits:
        problems.append(f"{metrics.forbidden_hits} forbid hit(s): a file that must never be found was")
    return problems


def render_report(
    label: str,
    embedder_id: str,
    today: str,
    metrics: Metrics,
    results: Sequence[QueryResult],
    sync: SyncReport,
    golden: Sequence[GoldenQuery],
) -> str:
    lines = [
        f"# Eval report: {label}",
        "",
        f"Date: {today}. Embedder: `{embedder_id}`. Golden queries: {metrics.n}. "
        f"Indexed: {sync.added} files, {sync.chunks_written} chunks "
        f"({sync.denied} denied, {len(sync.errors)} errors).",
        "",
        "| metric | value |",
        "|---|---|",
        f"| top1 | {metrics.top1:.3f} |",
        f"| recall@3 | {metrics.recall3:.3f} |",
        f"| recall@10 | {metrics.recall10:.3f} |",
        f"| MRR | {metrics.mrr:.3f} |",
        f"| forbidden hits | {metrics.forbidden_hits} |",
        "",
        "## Per query",
        "",
        "| # | query | first relevant rank | expected | top result |",
        "|---|---|---|---|---|",
    ]
    for number, (item, result) in enumerate(zip(golden, results, strict=True), start=1):
        rank = str(result.first_rank) if result.first_rank else "**miss**"
        top = result.top_uris[0] if result.top_uris else "-"
        lines.append(f"| {number} | {item.query} | {rank} | {', '.join(item.expect)} | {top} |")
    return "\n".join(lines) + "\n"


def run(argv: Sequence[str], make_embedder: Callable[[bool], Embedder], today: str) -> int:
    parser = argparse.ArgumentParser(prog="python -m agi_memory.eval")
    parser.add_argument("--data", default="../Sample Data")
    parser.add_argument("--golden", default="eval/golden.yaml")
    parser.add_argument("--report-dir", default="../docs/eval")
    parser.add_argument("--label", default="baseline")
    parser.add_argument("--cpu", action="store_true", help="embed on the CPU in float32")
    parser.add_argument("--write-baseline", action="store_true")
    args = parser.parse_args(list(argv))

    golden = load_golden(Path(args.golden))
    embedder = make_embedder(args.cpu)
    with tempfile.TemporaryDirectory() as work, threadpool_limits(limits=1):
        settings = Settings(index_dir=Path(work))
        store = FlatStore(Path(work) / "index", model_id=embedder.model_id, dim=embedder.dim)
        try:
            sync = build_index(store, embedder, settings, Path(args.data))
            results = evaluate(golden, Searcher(store, embedder))
        finally:
            store.close()
    metrics = summarize(results)

    report_dir = Path(args.report_dir)
    report_dir.mkdir(parents=True, exist_ok=True)
    report = render_report(args.label, embedder.model_id, today, metrics, results, sync, golden)
    (report_dir / f"{today}-{args.label}.md").write_text(report, encoding="utf-8")

    baseline_path = report_dir / "baseline.json"
    if args.write_baseline:
        text = json.dumps(baseline_dict(metrics, embedder.model_id), indent=2)
        baseline_path.write_text(text + "\n", encoding="utf-8")
        problems = [] if not metrics.forbidden_hits else [f"{metrics.forbidden_hits} forbid hit(s)"]
    elif baseline_path.exists():
        problems = compare_to_baseline(metrics, json.loads(baseline_path.read_text(encoding="utf-8")))
    else:
        problems = [] if not metrics.forbidden_hits else [f"{metrics.forbidden_hits} forbid hit(s)"]

    print(
        f"{metrics.n} queries: top1 {metrics.top1:.3f}, recall@3 {metrics.recall3:.3f}, "
        f"recall@10 {metrics.recall10:.3f}, MRR {metrics.mrr:.3f}, forbidden {metrics.forbidden_hits}"
    )
    for problem in problems:
        print(f"FAIL: {problem}")
    return 1 if problems else 0
```

`sidecar/src/agi_memory/eval/__main__.py`:

```python
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
```

- [ ] **Step 5: Run the tests, then the gate**

Run: `uv run pytest tests/test_eval.py -v`
Expected: 17 passed. If the baseline test's `top1 == 1.0` assertion fails, print `results` and check whether the fake embedder or keyword side failed to rank an obvious match first; the corpus is tiny, so it should hold. Fix the corpus text if a query is genuinely ambiguous.
Run: `uv run ruff check --fix . && uv run ruff format . && uv run ruff check . && uv run pyright && uv run pytest -q`
Expected: all pass. If pyright objects to `float(baseline[name])`, replace the `# type: ignore[arg-type]` with `float(str(baseline[name]))`.

- [ ] **Step 6: Commit**

```bash
git add sidecar/src sidecar/tests
git commit -m "feat(eval): golden queries, metrics, runner and CLI with a baseline gate"
```

### Task 11: Golden queries, first baseline, performance check and phase 4 gate

**Files:**
- Create: `sidecar/eval/golden.yaml`, `sidecar/tests/test_perf.py`, `docs/eval/baseline.json`, `docs/eval/<date>-baseline.md`, `docs/eval/perf.md`
- Modify: `docs/ARCHITECTURE.md`, `docs/STATUS.md`, `docs/README.md`

**Interfaces:** Consumes the CLI from Task 10, `Searcher`, `FlatStore`, `FakeEmbedder`.

- [ ] **Step 1: Create the golden queries**

`sidecar/eval/golden.yaml` (38 queries; URIs are relative to `Sample Data/`, so repo files start with `NextPlore/`):

```yaml
# Golden queries for the sample data. `expect`: a hit counts as relevant if its uri contains any
# of these strings. `forbid`: no top-10 hit may contain these (privacy checks).
# Written from the actual contents of Sample Data/ (code, docs, two statistics PDFs, images).

# --- code, natural language -------------------------------------------------------------
- query: find places near the user within a given radius
  expect: [locations.controller.js, models/location.js, locations.route.js]
- query: hash the user's password before saving it
  expect: [models/user.js]
- query: email a verification code to a new user
  expect: [emailService.js, user.controller.js]
- query: issue a new access token from a refresh token
  expect: [utils/jwt.js, user.controller.js, authService.js]
- query: map component that shows venue markers
  expect: [components/Map.jsx]
- query: many to many relation between locations and tags
  expect: [models/association.js, NextPlore/README.md, dataschemas.png]
- query: redirect logged in users away from the login page
  expect: [GuestRoute.jsx]
- query: protect pages so only authenticated users can open them
  expect: [ProtectedRoute.jsx]
- query: catch errors thrown while rendering React components
  expect: [ErrorBoundary.jsx]
- query: connect to PostgreSQL using Sequelize
  expect: [db/database.js, Backend/index.js]
- query: wrapper that catches errors in async express route handlers
  expect: [asynchandler.js]
- query: custom error class with an HTTP status code
  expect: [apiError.js]
- query: load country data from a CSV into the database
  expect: [addcountries.js]
- query: script that pulls foursquare places and creates tags
  expect: [updateLocationsAndTags.js, addlocations.js]
- query: page where a user resets a forgotten password
  expect: [ForgotPassword.jsx]
- query: search input that filters venues
  expect: [SearchBar.jsx, FilterPage.jsx]
- query: only let users in after their email address was verified
  expect: [UnverifiedRoute.jsx, VerifyEmail.jsx]

# --- exact names (keyword side) ---------------------------------------------------------
- query: addcountries
  expect: [addcountries.js]
- query: ProtectedRoute
  expect: [ProtectedRoute.jsx, App.jsx]
- query: emailService
  expect: [emailService.js]
- query: GEOGRAPHY POINT
  expect: [models/location.js, dataschemas.png, NextPlore/README.md]

# --- project documentation --------------------------------------------------------------
- query: how do I set up the backend environment variables
  expect: [NextPlore/README.md]
  forbid: [".env"]
- query: who built which part of the project
  expect: [NextPlore/README.md]
- query: contribution guidelines
  expect: [CONTRIBUTING.md]
- query: which database index makes the radius search fast
  expect: [NextPlore/README.md]

# --- privacy: secrets must never surface -------------------------------------------------
- query: JWT_ACCESS_SECRET
  expect: [NextPlore/README.md, utils/jwt.js]
  forbid: [".env"]

# --- PDFs (Week 10: confidence intervals; Week 11: Bayesian inference) -------------------
- query: how to calculate a confidence interval
  expect: [Week 10 Prep.pdf]
- query: standard error of the mean worked example
  expect: [Week 10 Prep.pdf]
- query: what changes when the population standard deviation is unknown
  expect: [Prep.pdf]
- query: table of critical values for the t distribution
  expect: [Week 10 Prep.pdf]
  note: page 9 has almost no extractable text, so only the page image can match well
- query: prior distribution times likelihood gives the posterior
  expect: [Week 11 Prep.pdf]
- query: bayesian updating of the mean as more data points are seen
  expect: [Week 11 Prep.pdf]

# --- images (matched by content, not by name) -------------------------------------------
- query: entity relationship diagram of the database tables
  expect: [dataschemas.png]
- query: which columns does the users table have
  expect: [dataschemas.png, models/user.js]
- query: n8n workflow for collecting data from the foursquare api
  expect: [datapipeline.png]
- query: app logo white text with a red arrow on black
  expect: [Logo V1.png]
- query: man hiking on a mountain at night under the stars
  expect: [WhatsApp Image]
- query: football player in a white kit standing in the rain
  expect: [cristiano-ronaldo]
```

- [ ] **Step 2: Run the harness on the real model and record the baseline**

Run (from `sidecar`): `uv run python -m agi_memory.eval --write-baseline`
Expected: loads EmbeddingGemma 2 on the GPU (about 15 s), indexes `Sample Data/` (about 400 chunks, about 20 s), prints a one-line summary and writes `../docs/eval/<today>-baseline.md` and `../docs/eval/baseline.json`; exit status 0 (forbid hits would make it 1: if any appear, that is a privacy defect, stop and fix it before continuing).

- [ ] **Step 3: Inspect the report; judge each miss**

Read `docs/eval/<today>-baseline.md`. For every query whose first relevant rank is a miss or worse than 3, decide with evidence: (a) the expectation is factually wrong (the answer really lives in another file; fix `golden.yaml` and re-run), or (b) retrieval is genuinely weak (leave the query; this is the baseline we improve in phase 6). Do not tune retrieval or loosen expectations to raise numbers in this task. Write the decisions into the report's end as a short "Reading the baseline" section (3-8 bullets: what is strong, what is weak, the likely cause of each weak group).

- [ ] **Step 4: Performance test and record**

`sidecar/tests/test_perf.py`:

```python
import time

import numpy as np
import pytest

from agi_memory.embedding import FakeEmbedder
from agi_memory.search.searcher import Searcher
from agi_memory.store import FlatStore
from agi_memory.types import Chunk, ChunkKind

pytestmark = pytest.mark.perf

N = 100_000
DIM = 256
BATCH = 1000
VOCAB = [f"word{i}" for i in range(3000)]


def test_hybrid_search_over_100k_chunks_stays_under_50_ms_p95(tmp_path):
    rng = np.random.default_rng(7)
    embedder = FakeEmbedder(DIM)
    store = FlatStore(tmp_path / "idx", model_id=embedder.model_id, dim=DIM)
    try:
        for start in range(0, N, BATCH):
            chunks = [
                Chunk(
                    "perf", f"f{start + i}.txt", ChunkKind.TEXT, "{}", f"h{start + i}", 1,
                    f"f{start + i}.txt\n" + " ".join(rng.choice(VOCAB, 30)),
                )
                for i in range(BATCH)
            ]  # fmt: skip
            raw = rng.standard_normal((BATCH, DIM)).astype(np.float32)
            store.upsert(chunks, raw / np.linalg.norm(raw, axis=1, keepdims=True))
        searcher = Searcher(store, embedder)
        queries = [" ".join(rng.choice(VOCAB, 4)) for _ in range(200)]
        searcher.search(queries[0])  # warm up
        timings = []
        for query in queries:
            started = time.perf_counter()
            searcher.search(query)
            timings.append((time.perf_counter() - started) * 1000)
        p50, p95 = np.percentile(timings, [50, 95])
        print(f"100k chunks: p50 {p50:.1f} ms, p95 {p95:.1f} ms")
        assert p95 < 50
    finally:
        store.close()
```

Run: `uv run pytest -m perf -s -q`
Expected: passes and prints `p50`/`p95` (the build of 100k rows takes about a minute). If `p95 >= 50 ms`, profile (`search_vectors`, `keyword_search`, `store.get`) and report the numbers to the owner with the dominant cost; do not relax the 50 ms bound.
Create `docs/eval/perf.md` recording: date, machine (RTX 4050 laptop, CPU search), N = 100,000, 256 dims, the printed p50/p95, and the sentence "Query embedding (~200 ms with the real model) is excluded; see ADR 0001 for raw vector-search timings."

- [ ] **Step 5: Phase 4 docs gate**

`docs/ARCHITECTURE.md`: set the `search/` and `eval/` rows to `built`, update their jobs ("`Searcher`: vector + FTS5 keyword search, reciprocal rank fusion, filters" and "Golden queries, metrics, runner/CLI (`uv run python -m agi_memory.eval`), baseline gate"); add to "Invariants": "7. **Retrieval quality is gated.** `docs/eval/baseline.json` holds the accepted metrics; `uv run python -m agi_memory.eval` fails if any metric drops more than 0.02 or a `forbid` query surfaces a file it must not." `docs/STATUS.md`: phase 4 `done`, record the baseline numbers (top1, recall@3, recall@10, MRR) and the p95, and set "Next" to "Plan phase 5 (HTTP API) and phase 6 (quality iteration, driven by the weak groups in the baseline report)". `docs/README.md`: change the `eval/` row to say it exists: "Retrieval-quality reports, `baseline.json`, `perf.md`".

- [ ] **Step 6: Final gate and commit**

Run: `uv run ruff check . && uv run ruff format --check . && uv run pyright && uv run pytest -q` (all clean), then `uv run pytest -m model -q` (the real-model tests still pass) and `git status --short` (nothing under `Sample Data/` or any `.env`).

```bash
git add sidecar/eval sidecar/tests docs
git commit -m "feat(eval): 38 golden queries, first baseline and 100k performance check"
```

---

## Self-Review (done against the spec)

**Spec coverage (phases 3-4).** Phase 3: ignore rules and a non-overridable deny list (T1, T2), change detection by `(size, mtime_ns)` then SHA-256 (T6), baseline chunking for code, Markdown, text, PDF text + page image, images (T3, T4), failure policy (T6), batching (T6), "re-index of an unchanged folder embeds nothing; `.env` provably never read" (T6, T7). Phase 4: FTS5 index and filters (T8), RRF with id tie-break and the response-hit fields (T9), eval harness with golden set, metrics, baseline gate, report, CPU/BLAS pinning (T10, T11), p95 < 50 ms at 100k synthetic chunks (T11). Deliberately deferred and recorded: `jobs.py` and the HTTP API (phase 5), tree-sitter chunking, 768-d rerank, cross-encoder reranker and the "no good answer" signal (phase 6, driven by the baseline), a CPU-float32 vs GPU eval gap measurement (run the CLI once with `--cpu` after the baseline and note the difference in the report).

**Placeholders.** None: each step contains its code or exact commands. Values that only exist after running (baseline numbers, p50/p95) are recorded into docs by explicit steps, not assumed.

**Type consistency.** `ScannedFile`/`ScanResult.unreadable` (T2) are consumed with the same names in `Indexer.sync` (T6); `FileRecord(source_id, uri, size, mtime_ns, sha256)` is constructed identically in T5, T6 and the test helpers; `chunk_text_document(source_id, uri, route, data, mtime_ns, settings)` matches its call in T6; `pdf_text_chunks(source_id, uri, pdf, mtime_ns, settings)` and `pdf_page_chunks(source_id, uri, file_sha256, page_count, mtime_ns)` match T6; `KeywordHit`/`SearchHit` fields match T8-T10; `reciprocal_rank_fusion(rankings, k)` and `Searcher.search(query, k, filters)` match the eval runner; `SyncReport` fields used by `render_report` exist.

**Known soft spots to watch while executing.** The Task 6 spy test patches `builtins.open` and `io.open`; if pathlib reaches files through another route the test must be extended, not weakened. The Task 8 tie test relies on bm25 returning identical floats for identical text. The Task 11 golden expectations were written from reading the files, so Step 3 exists to correct any that are factually wrong.
