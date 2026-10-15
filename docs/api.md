# Memory engine HTTP API

The contract the Go launcher codes against. Source of truth for behaviour is `sidecar/src/agi_memory/api/`; the
tests in `sidecar/tests/test_api_*.py` pin every status code listed here.

## Process contract

Start from `sidecar/`:

```
uv run python -m agi_memory
```

| Environment variable | Meaning |
|---|---|
| `AGI_SIDECAR_TOKEN` | Required. At least 16 characters. Generate a fresh random value per launch. |
| `AGI_SIDECAR_PORT` | Optional, 1-65535, default `7861`. |
| `AGI_MEMORY_DIR` | Optional. Index location; default `%LOCALAPPDATA%\AGI\memory`. |

- A corrupt or mismatched index (wrong model or dimension) makes the process exit with a traceback instead of
  answering `/health`: the launcher should treat an early exit as fatal and offer a reindex.
- Bad configuration exits with status 2 and one line on stderr, before anything heavy is imported (about a second).
- The port opens within about a second. The embedding model then loads in the background (10-15 s warm).
  Poll `GET /health` until `ready` is `true`. If `error` is non-null the model failed to load and `ready` will never
  become true: show the error and restart. The process stays alive in that case.
- Stop by terminating the process. Attach it to a Windows Job Object so it cannot outlive the Go process. The index
  survives a hard stop (crash-safety is tested).

## Security

- Binds `127.0.0.1` only.
- Every route, including `/health`, needs `Authorization: Bearer <token>`. A wrong or missing token is `401`
  (also when the body is invalid: 401 wins over 422). The token is compared in constant time and never echoed.
- The `Host` header must be `127.0.0.1` or `localhost` (any port); anything else is `400` before the token is
  looked at (DNS-rebinding defence). Use `127.0.0.1` in URLs: `[::1]` is rejected.
- No CORS headers, no `/docs`, `/redoc` or `/openapi.json` (they answer 404).

## Errors

Error bodies are `{"detail": ...}`. `detail` is a string, except `409` from `POST /sources` (an object, see below)
and `422` (FastAPI's list of validation problems). A foreign `Host` gets a plain-text `400`.
`503` means the model is not loaded (yet): `detail` says whether it is still loading or failed to load.

## Routes

### `GET /health`

`200`:

```json
{"ready": true, "model": "google/embeddinggemma-2", "device": "cuda", "embedded_items": 1234,
 "version": "0.1.0", "error": null}
```

`ready` turns true once the model is loaded. `device` is `null` until then. `error` is `null` unless the model
failed to load. `embedded_items` is the number of chunks in the index and works even before the model is ready.
`/health` is the only route that answers while the model is loading.

### `POST /sources`

Index a folder (or refresh it if it was indexed before: the sync is incremental).

Request: `{"path": "C:\\Users\\me\\project", "source_id": "project"}`. Unknown fields are rejected.

- `202` `{"job_id": "job-1"}`
- `400` `path` is not an absolute local path to an existing folder. Network (UNC) paths are refused.
- `409` another job is running: `{"detail": {"message": "job-1 is still running", "job_id": "job-1"}}`. Follow that
  job instead. Also `409` (with `"job_id": null`) when `source_id` was already indexed from a different folder:
  `DELETE` it first, so a typo cannot wipe an index. The same folder written differently (case, slashes) is a refresh.
- `422` bad body, or `source_id` does not match `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`, or `path` is empty / over 1024
  characters.
- `503` model not ready.

### `GET /sources`

What is indexed: `200` a list, ordered by `id` (empty when nothing is indexed):

```json
[{"id": "project", "root": "C:\\Users\\me\\project", "last_scan_ns": 1760000000000000000, "files": 96, "chunks": 431}]
```

`root` is the folder as it was submitted to `POST /sources`, `last_scan_ns` is `0` until the first sync of that source has
finished, `files` and `chunks` count what the index holds now. This route only reads the index, so unlike the others it
answers while the model is still loading.

### `GET /jobs/{job_id}`

`200`:

```json
{"id": "job-1", "source_id": "project", "state": "running", "files_done": 12, "files_total": 40,
 "report": null, "error": null}
```

`state` is `running`, `done` or `failed`. `files_total` is `0` while the folder is still being scanned; progress counts
start once the scan finishes. When `done`, `report` is:

```json
{"added": 3, "updated": 0, "unchanged": 0, "removed": 0, "denied": 1, "ignored": 0, "unsupported": 0,
 "too_large": 0, "chunks_written": 3, "errors": [{"uri": "broken.png", "message": "..."}]}
```

`errors` lists files that could not be read or decoded; every other file is still indexed. `denied` counts files
the secret deny list refused to open (`.env`, keys, credential JSON, ...). When `failed`, `error` is
`"<ExceptionType>: <message>"` (for example the folder disappeared) and `report` is `null`.

- `404` unknown job (also after a restart, or once more than 50 newer jobs exist).
- `503` model not ready.

### `DELETE /sources/{source_id}`

Remove every chunk of a source. `200` `{"removed_chunks": 3}`. Idempotent: deleting an unknown or already deleted
source is `200` with `0`.

- `409` that source is being indexed right now.
- `422` invalid `source_id`.
- `503` model not ready.

### `POST /search`

Request:

```json
{"query": "how do I refresh the jwt token", "k": 10,
 "filters": {"kinds": ["code"], "source_id": "project", "uri_prefix": "Backend/", "modified_after_ns": 0}}
```

`query` is up to 4000 characters; empty or blank returns no hits. `k` is 1-100, default 10. Every filter is
optional, and so is `filters`. `kinds` values: `code`, `markdown`, `text`, `pdf_text`, `pdf_page`, `image`.
`modified_after_ns` is integer nanoseconds since the epoch (the store's unit). Unknown fields are rejected.

`200`:

```json
{"hits": [{"id": "...", "source_id": "project", "uri": "Backend/jwt.js", "kind": "code", "locator": "{\"lines\":[1,40]}",
           "score": 0.81, "vector_score": 0.79, "keyword_rank": 1, "snippet": "..."}],
 "confident": true}
```

`confident` is `false` when the best hit is probably noise: its cosine is below 0.65 and the query is not part of its
path. Show "no match" instead of the list; the hits are still returned. On the sample corpus every on-topic query was
confident and 14 of 15 off-topic ones were not. It is a flag calibrated on one corpus, not a guarantee.

`uri` is relative to the source root with `/` separators. `locator` is JSON text (`{"lines":[a,b]}`, `{"page":n}`,
...). `score` is cosine similarity plus small keyword and file-name boosts (about 0.5 to 1.0 for real matches): use it to
order and compare within one response, not as a probability. `vector_score` is cosine similarity,
or `null` if the vector search did not return the chunk; `keyword_rank` is the 1-based keyword rank, or `null`.
Scores are not calibrated across corpora; use `confident` for "nothing relevant".

- `422` invalid body (`k` out of range, unknown kind, over-long query, ...).
- `503` model not ready.

### `POST /search/files`

The same search, shown as files: each file once, in the order of its best chunk, with its best matching chunks. This is
what a result list wants; `POST /search` is the chunk-level view.

Request: `{"query": "...", "k": 10, "per_file": 3, "filters": {...}}`. `k` is the number of files (1-50, default 10),
`per_file` the number of chunks kept per file (1-10, default 3); `query` and `filters` are as for `/search`. Unknown fields
are rejected.

`200`:

```json
{"files": [{"source_id": "project", "uri": "Backend/jwt.js", "kind": "code", "score": 0.81,
            "chunks": [{"id": "...", "source_id": "project", "uri": "Backend/jwt.js", "kind": "code",
                        "locator": "{\"lines\":[1,40]}", "score": 0.81, "vector_score": 0.79,
                        "keyword_rank": 1, "snippet": "..."}]}],
 "confident": true}
```

`score` and `kind` of a file are those of its best chunk (`pdf_text` or `pdf_page` for a PDF), and `chunks` are best first.
The search ranks at least 60 chunks before grouping, so a file with many matching chunks does not push other files out.
A file's absolute path is the root of its source plus `uri`; the engine does not know roots' meaning beyond what was
submitted to `POST /sources`.

- `422` invalid body. `503` model not ready. `confident` has the same meaning as for `/search`.

## Semantics worth knowing

- One index job at a time. A search issued while indexing is served normally: the job and the search take turns on
  the one model. Bulk embedding runs in slices of 4 texts or 1 image and queued queries go first, so a search waits for one
  slice, not for the whole batch: on the sample folder a search during a job takes about 150 ms median and 250-310 ms at the
  95th percentile (idle: about 80 ms), at a cost of roughly 10-15% indexing time.
- Posting an already-indexed folder again is an incremental sync: unchanged files are skipped, changed ones
  replaced, deleted ones removed. That is the refresh mechanism; nothing watches the disk.
- Job history lives in memory (last 50) and is lost on restart. The index itself is durable.
- Paths must be absolute local paths and exist. Secrets are never indexed (deny list; counted in `report.denied`).

## Deviations from the spec

- `/health` also returns `error` (why the model failed to load).
- `POST /sources` refuses to re-point an existing `source_id` at another folder (409).
- `DELETE /sources/{id}` returns `{removed_chunks}` and is idempotent.
- `/docs`, `/redoc` and `/openapi.json` are disabled (they would be unauthenticated pages).
- IPv6 hosts (`[::1]`) are rejected; clients must use `127.0.0.1`.
- The spec's `IndexJob` is `JobManager` / `Job` in `jobs.py`.
- The `Embedder` contract gained a read-only `device` property.
- The filter `modified_after` is `modified_after_ns`, integer nanoseconds like the store.
