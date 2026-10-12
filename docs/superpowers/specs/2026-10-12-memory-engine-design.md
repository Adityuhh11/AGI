# Memory Engine: Design Spec

Date: 2026-10-12. Status: approved 2026-10-12 (search runs on the CPU; the GPU only hosts the embedding model). Decisions inherited: [ADR 0001](../../decisions/0001-vector-store.md).

## 1. Purpose

A local service (the Python **sidecar**) that indexes the owner's files and answers "find the thing I mean" queries across
code, documents, PDFs and images, with the highest retrieval quality we can measure. It is the foundation for every
later feature (launcher search, RAG for the quick-ask panel, drop shelf, screen memory). The Go launcher is a separate
spec; this spec ends at a stable, authenticated localhost HTTP API.

### Success criteria

1. Indexing the sample data (`Sample Data/`, git-ignored) and running the golden query set meets the recorded baseline,
   and no later change lowers it without an explicit, documented reason.
2. Search latency at 100k items: **p95 < 50 ms** for the vector+keyword pipeline (excluding query embedding, which is
   ~200 ms per call and measured separately).
3. Re-indexing an unchanged folder does no embedding work and leaves the set of chunk ids, their vectors and their
   metadata unchanged (scan timestamps excepted).
4. A crash at any point leaves an index that opens and searches correctly (possibly missing the last in-flight batch).
5. Every module can be understood from its docstring and `docs/ARCHITECTURE.md` without reading the others.

### Non-goals (this spec)

Go/UI work, audio and video, file watching, multi-vector (late interaction), approximate (ANN) indexes, GPU-side search (benchmarked in ADR 0001, not adopted: VRAM stays free for models), multi-user,
network access beyond localhost, LLM calls. Each is a later spec; the interfaces below must not block them.

## 2. Constraints

- Windows 11, RTX 4050 with **6 GB VRAM** shared with a future LLM. The sidecar uses the GPU **only for the embedding
  model** (0.8-1.4 GB measured); index storage and search run on the CPU and system RAM.
- Privacy: nothing leaves the machine. No telemetry. Secrets are never indexed (section 6).
- Determinism: same inputs produce the same chunk ids, the same stored rows and the same ranked results (section 9).
- Python managed by `uv`; torch from the CUDA index (already configured in `sidecar/pyproject.toml`).

## 3. Architecture

```
            Go launcher (later)                       sidecar/  (this spec)
 hotkey/UI ───────HTTP+token───────►  api  ──►  jobs ──► sources ──► chunking ──► embedding
                                       │                                              │
                                       └──► search pipeline ◄── store (SQLite + vector file) ◄─┘
                                              vector top-N + keyword top-N → RRF → (rerank) → results
```

Data flow, indexing: `Source` yields `Document`s (file bytes + metadata) → `Chunker` turns each into `Chunk`s (text or
image) → `Embedder` produces 256-d unit vectors → `Store.upsert(chunks, vectors)`.
Data flow, search: query → `Embedder.embed_query` + FTS5 query → two ranked lists → fusion → results with scores,
source path, locator.

## 4. Modules

Package `sidecar/src/agi_memory/`. One module = one job. Dependencies point downward only (api → search/jobs →
store/embedding/chunking/sources → core types).

| Module | Job | Key types |
|---|---|---|
| `types.py` | Plain data: `Chunk`, `ChunkKind`, `SearchHit`, `Filters` (frozen dataclasses) | |
| `config.py` | Paths, model id, dimensions, limits; one frozen `Settings`, env overrides | `Settings` |
| `embedding/` | `Embedder` protocol; `GemmaEmbedder` (real); `FakeEmbedder` (hash-based, tests) | `embed_documents`, `embed_images`, `embed_query` |
| `chunking/` | `chunk(document) -> list[Chunk]` by file type | `code.py`, `markdown.py`, `pdf.py`, `image.py` |
| `sources/` | `FolderSource`: walk, ignore rules, change detection | `Document`, `scan()` |
| `store/` | Persistence and exact vector search | `Store` protocol; `FlatStore` |
| `search/` | Pipeline: vector + keyword + fusion (+ rerank hook) | `Searcher` |
| `jobs.py` | Background indexing with progress; one job at a time | `IndexJob` |
| `api/` | FastAPI app, auth, request/response models | |
| `eval/` | Golden-set loader, metrics, runner, report | CLI `python -m agi_memory.eval` |

`Embedder` and `Store` are protocols because both have a second, deterministic implementation we need anyway
(`FakeEmbedder` for tests; Chroma and the NumPy oracle for benchmarks). Nothing else gets an interface until it has two
implementations.

## 5. Data model and storage

Index directory: `%LOCALAPPDATA%\AGI\memory\` (override via `AGI_MEMORY_DIR`, used by tests). Two files plus SQLite WAL files.

**`meta.db` (SQLite, WAL mode, source of truth)**

- `chunks(rid INTEGER PRIMARY KEY, id TEXT UNIQUE, vec_row INTEGER UNIQUE, kind TEXT, source_id TEXT, uri TEXT,
  locator TEXT, content_hash TEXT, mtime_ns INTEGER, text TEXT, identifiers TEXT)`. `id` = sha256 of `source_id + uri + locator + content_hash` (hex, `\0`-separated). `vec_row` is the chunk's row in the
  vector file (`row` is a SQLite keyword). `kind` in `code | markdown | pdf_text | pdf_page | image | text`. `locator` is JSON (`{"lines":[1,40]}`,
  `{"page":3}`).
- `sources(id, root, last_scan_ns)`; `files(source_id, uri, size, mtime_ns, sha256)` for change detection.
- `chunks_fts`: FTS5 external-content table over `text` plus an `identifiers` column (camelCase and snake_case split into
  words, original kept), tokenizer `unicode61 remove_diacritics 2`. Verified available (SQLite 3.49.1 here).
- `meta(key, value)`: schema version, embedding model id, dimension, dtype. Opening an index whose model id or dimension
  differs from `Settings` is an error ("reindex required"), never a silent mix.

**`vectors.f32`**: a memory-mapped matrix of shape `(capacity, 256)`, float32, rows L2-normalised, with a fixed 64-byte
header (magic, version, dimension, capacity). Capacity doubles when full. Deleted chunks free their row for reuse.
Float32 (not float16) because a float32 matrix multiply is the fast CPU path (4.3 ms at 100k, 47 ms at 1M measured); the
cost is 1 KB per chunk on disk and in RAM (1M chunks = 1 GB).

**Write ordering (crash safety).** For each batch: (1) write vectors into free rows and flush; (2) in one SQLite
transaction insert/update `chunks`, `files` and FTS. A vector row with no `chunks` entry is garbage and is overwritten
later; a `chunks` entry always points at a flushed row. Opening the store never needs repair.

**Search.** Exact search on the CPU: one float32 matrix multiply of the query against the memory-mapped matrix,
`argpartition` for the top-`N`, then a sort by `(-score, id)`. No GPU copy of the index exists, so VRAM stays free for
models. Beyond ~5M chunks an approximate index (`hnswlib`) becomes a separate spec.

## 6. Ingestion

- **Ignore rules**: honour `.gitignore` files, plus a built-in deny list that cannot be overridden: `.git/`,
  `node_modules/`, `.venv/`, `__pycache__/`, `.env*`, `*.pem`, `*.key`, `id_rsa*`, `*.p12`, lockfiles, files over 5 MB
  (images/PDFs exempt up to 50 MB). Denied files are skipped and counted in the job report, never read.
- **Change detection**: `(size, mtime_ns)` first, then SHA-256 only when those differ. Unchanged files cost no embedding.
  Files that disappeared have their chunks deleted. Moves are treated as delete + add (content-hash dedupe is a later
  optimisation).
- **Chunking (baseline)**: code and text, 40-line windows with 10-line overlap, embedded text is
  `"<relative path>\n<chunk>"`. Markdown: split by heading, then by size. PDF: one `pdf_text` chunk and one `pdf_page`
  image chunk per page (page rendered at 1.5x). Image: thumbnailed to 1024 px (EXIF orientation applied), kind `image`, chunk text = the file's relative path so file names are keyword-searchable; `pdf_page` chunks have empty text (their words live in the page's `pdf_text` chunk). Token cap 512 per chunk.
  Better code chunking (tree-sitter symbols) is a measured improvement in phase 6, not a baseline assumption.
- **Batching**: text batch 8, image batch 4 (largest sizes that fit the budget in the spike; revisit with measurements).
- **Failure policy**: a file that fails to parse is recorded in the job report with the error and skipped; the job
  continues. A failure writing the store aborts the job (never half-applies a batch).

## 7. Search

1. Embed the query (`SearchQuery` prompt, 256-d). Build keyword query from the same text (tokens quoted, identifiers split).
2. **Vector**: exact top-`N` (default 200) by dot product over rows allowed by filters.
3. **Keyword**: FTS5 `bm25` top-`N` with the same filters.
4. **Fusion**: reciprocal rank fusion, `score = sum(1 / (60 + rank))` over both lists, ties broken by chunk id so results
   are deterministic. Top-`k` returned (default 10).
5. **Filters** (`kind`, `source_id`, `uri_prefix`, `modified_after`): resolved in SQLite to a set of rows, applied as a
   mask on the vector side and a join on the keyword side.
6. **Response hit**: `{id, uri, kind, locator, score, vector_score, keyword_rank, snippet}`. Raw scores are exposed because
   cosine scores are not calibrated (nonsense queries scored 0.655 vs 0.74-0.85 for real hits). A "no good answer"
   signal is a phase 6 item decided from eval data; it is deliberately not in API v1.
7. A rerank hook (`Reranker` protocol, default none) sits between fusion and the response for phase 6.

## 8. HTTP API (localhost only)

Binds `127.0.0.1` on a configurable port (default 7861). Every request needs `Authorization: Bearer <token>`; the token
is generated by the Go process at launch and passed in env `AGI_SIDECAR_TOKEN`. Requests with a non-local `Host`
header are rejected (blocks DNS-rebinding from web pages). The server refuses to start without a token.

| Method, path | Purpose |
|---|---|
| `GET /health` | `{ready, model, device, embedded_items, version}`; `ready` only after the model is loaded |
| `POST /sources` `{path, source_id}` | Register a folder and start an index job; returns `job_id` |
| `GET /jobs/{id}` | Progress, counts (added/updated/deleted/skipped), per-file errors |
| `DELETE /sources/{source_id}` | Remove a source and all its chunks |
| `POST /search` `{query, k?, filters?}` | Ranked hits (section 7) |

Process contract for the Go side: start with `uv run python -m agi_memory`, poll `/health` until `ready`, stop by
terminating the process (Go attaches it to a Windows Job Object so it cannot outlive Go). Startup time is dominated by
model load (~9-15 s warm), so the launcher must never block on it.

## 9. Determinism and correctness

- **IDs are content-derived** (section 5), so re-indexing is idempotent.
- **Tests never use the GPU or the real model by default.** `FakeEmbedder` maps text to a vector by hashing, so a given
  text always yields the same vector and unit tests are exact. Tests that need the real model are marked
  `@pytest.mark.model` and run on demand.
- **Ordering**: every ranked list breaks ties by chunk id. Directory walks are sorted.
- **Oracle**: `FlatStore` search is compared against a plain NumPy brute-force implementation on randomised stores
  (seeded), including after deletes, upserts and reopen.
- **Reproducible eval**: the model runs in bf16 on the GPU, which can differ slightly from CPU float32. The eval harness
  can run the embedder on the CPU in float32 (`--cpu`) for exact reproducibility; the gap between the two modes is measured
  once and recorded. BLAS thread count is pinned to 1 for eval runs so summation order is stable.
- Time and randomness are injected (no hidden `time.time()` or unseeded random in logic code).

## 10. Evaluation harness

`sidecar/eval/golden.yaml`: entries `{query, expect: [uri substrings], note}`. Seeded with the 12 queries from the
2026-10-12 spike (expectations corrected: `addlocations.js` is an acceptable answer for the Foursquare query). The owner
adds 5-10 real queries in their own words. Metrics: top-1, recall@3, recall@10, MRR, plus per-query ranks. The runner
indexes `Sample Data/` into a temporary directory, runs every query, writes a report to `docs/eval/<date>-<label>.md`, and
exits non-zero if any headline metric drops more than 0.02 below `docs/eval/baseline.json`. Raising the baseline is an
explicit commit.

## 11. Testing and quality gates

- `pytest` (unit, store oracle, crash recovery by killing between the vector write and the SQLite commit, API tests
  with FastAPI `TestClient`), `ruff check` and `ruff format --check`, `pyright` on `src/`.
- A phase is done only when: tests pass, lint/type checks are clean, docs for that phase are updated in the same commits,
  and the eval harness has not regressed (once it exists, phase 4 onward).
- Performance checks (separate marker): 100k-item search latency and indexing throughput recorded in
  `docs/eval/perf.md` so regressions are visible.

## 12. Documentation for humans and agents

Shared context lives in the repo, not in anyone's head:

- `AGENTS.md` (repo root): the single source of project rules for any coding agent; `CLAUDE.md` only imports it
  (`@AGENTS.md`). It tells agents to read `docs/README.md` and `docs/STATUS.md` first.
- `docs/README.md`: index of every doc and what it answers.
- `docs/ARCHITECTURE.md`: living description of modules, data flow, on-disk formats, invariants. Updated with the code.
- `docs/STATUS.md`: what is done, what is next, known gaps; updated at the end of every phase.
- `docs/decisions/NNNN-*.md`: ADRs for choices with trade-offs.
- Module docstrings state the module's job, its inputs/outputs and invariants. Public functions have docstrings that say
  *why*, not *what*.

## 13. Build phases

Each phase is a sequence of small, reviewable commits (conventional commits, one logical change each, tests in the same
commit as the code they cover). Each ends with the gate in section 11 and a `docs/STATUS.md` update.

| Phase | Deliverable | Gate (beyond section 11) |
|---|---|---|
| 0 Foundations | Package skeleton, `Settings`, ruff/pyright/pytest wiring, docs scaffold (`docs/README.md`, `ARCHITECTURE.md`, `STATUS.md`) | `pytest` runs on an empty-but-real suite |
| 1 Embedding | `Embedder` protocol, `FakeEmbedder`, `GemmaEmbedder` (text + image, bf16/GPU, CPU fallback) | Real-model test reproduces spike similarities within tolerance; VRAM <= 1.5 GB |
| 2 Store | Schema, vector file, `FlatStore` (exact CPU search), reopen/crash safety | Oracle tests green; kill-between-writes test green |
| 3 Ingestion | Ignore rules, chunkers (baseline), `FolderSource`, incremental sync, jobs | Re-index of unchanged folder embeds nothing; `.env` provably never read |
| 4 Search + eval | FTS5 index, filters, RRF, `Searcher`, eval harness, golden set, first baseline | Baseline recorded; p95 < 50 ms at 100k synthetic items |
| 5 API | FastAPI app, token + host checks, `/health`, `/sources`, `/jobs`, `/search`, run entry point | API tests green; start/ready/stop contract documented |
| 6 Quality iteration | Each experiment is its own commit, adopted only if eval improves: tree-sitter chunking, 768-d rerank stage, cross-encoder reranker, "no good answer" signal | Eval delta recorded in `docs/eval/` per experiment |

Phases 0-5 are the committed scope. Phase 6 items are ordered hypotheses, not promises.

## 14. Risks

| Risk | Mitigation |
|---|---|
| bf16 GPU embeddings differ slightly from CPU float32 ones | Tie-break by id; optional CPU-float32 eval mode; measure the gap once |
| CPU search cost grows with item count | 47 ms at 1M measured; revisit at ~5M (ANN spec); float32 index costs 1 KB of RAM per chunk |
| `sentence-transformers` / `transformers` major-version API drift (this model is new) | Pin versions in `uv.lock`; real-model smoke test in phase 1 |
| Index corruption | SQLite as source of truth, write ordering in section 5, crash test |
| Indexing a secret | Built-in deny list (not overridable), tested; deny list documented in ARCHITECTURE.md |
| Scope creep | Non-goals list; Phase 6 gated by eval |
