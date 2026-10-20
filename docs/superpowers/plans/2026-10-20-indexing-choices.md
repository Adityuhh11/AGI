# Plan: indexing choices (PDF pages, photos, videos)

Spec: `../specs/2026-10-20-indexing-choices-design.md`. Branch `feat/indexing-choices`. Each block ends green
(`uv run pytest`, `ruff`, `go vet`, `go test ./...`, `npm test`, `npm run check`) and is one commit.

## A. Engine: reuse by chunk id
1. `FlatStore.vectors_for(ids)`: stored vectors by chunk id.
2. Indexer: text flushes, PDF text and page chunks and image chunks embed only ids the store lacks.
3. Tests: re-syncing a changed file embeds only its new chunks; a counting embedder sees nothing for an unchanged chunk.

## B. Engine: options
1. `IndexOptions(pdf_pages, photos, videos)` in `types.py`; `source_options` table; `FlatStore.source_options` / `set_source_options`.
2. `POST /sources` takes `options`; `JobManager.submit` and `Indexer.sync` pass them; stored after a finished sync.
3. Indexer: kinds whose option changed are re-processed even when unchanged on disk.
4. PDF: `page_needs_image` (images, characters, path objects); `auto`, `all`, `text`.
5. Photos `name`: an image chunk embedded from its name (`name:` content hash).
6. Tests for each.

## C. Engine: videos
1. `av` in `pyproject.toml` / `uv.lock`; the install check imports it.
2. `Route.VIDEO`, `ChunkKind.VIDEO` and `VIDEO_FRAME`; scanner: no size limit; identity hash of size + first and last MiB.
3. `chunking/video.py`: duration and frames by seeking; name chunk; frame chunks.
4. Indexer: name chunk always, frames when `videos = content`; a broken video keeps its name.
5. Tests on a clip generated with PyAV.

## D. Launcher
1. `config.Search`: `PDFPages`, `Photos`, `Videos` with defaults, validation, old files load.
2. `sidecar.AddSource` sends options; `indexing.Manager.SetOptions` (a change starts a sync).
3. `search`: `video` kind, `vid:` scope, "at 1:23".
4. Settings: `SetIndexing(pdf, photos, videos)`, device in `SearchView`, folder reasons (nothing readable, moved known folder).
5. Frontend: "What to index" on the File search page; video icon in the bar.

## E. Measure and document
Golden eval with defaults; sample and PDF index times `auto` against `all`; STATUS, ARCHITECTURE, api.md, eval note.
