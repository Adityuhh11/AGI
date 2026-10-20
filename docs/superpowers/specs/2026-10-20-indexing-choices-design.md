# Indexing choices: PDF pages, photos and videos

Date: 2026-10-20. Status: approved by the owner in conversation; building on `feat/indexing-choices`.

## Why

AGI is open source now, so it runs on machines very unlike the owner's laptop. Measured on that laptop
(`docs/eval/2026-10-20-indexing-throughput.md` and below):

| | RTX 4050 (Vulkan) | Intel UHD (Vulkan) | CPU only |
|---|---|---|---|
| one picture or PDF page | about 0.16 s | 4.8 s | 4.9 s |
| text | about 53 chunks/s | 2.6 chunks/s | 1.4 chunks/s |

Pictures are what costs: on the owner's index, 17,710 picture vectors (16,713 of them PDF pages) against 28,678 text
chunks. Without a discrete GPU the PDF pages alone would take about 22 hours. People should choose what is worth that
time on their machine, and the defaults should be sensible on every machine.

Videos are not indexed at all today. A spike (PyAV 19.0.1, 10 real videos from 5 s to 30 min): 8 frames decode in 0.25 to
0.5 s whatever the length (seeking, not decoding everything), embed in 1.3 s on the GPU, and frames alone found the right
video for 5 of 6 queries that describe what is in it ("red ball floating in a river", "politician giving a speech").

## What the person chooses

Three settings in `[search]` of `config.toml`, shown on the File search page under "What to index", each with one sentence
saying what it costs:

| Setting | Values (default first) | Meaning |
|---|---|---|
| `pdf_pages` | `auto`, `all`, `text` | `auto`: a page is also searched by how it looks when it has a picture, less than 200 characters of text (scans, slides) or more than 50 drawn shapes (charts, tables, diagrams); every page is searched by its words. `all`: every page by how it looks too. `text`: words only. |
| `photos` | `content`, `name` | `content`: pictures are searched by what they show. `name`: by their file name only (one short text embedding each). |
| `videos` | `name`, `content` | `name`: videos are found by their file name. `content`: also by what is in them, 8 frames each. |

The `auto` rule, measured on the owner's 16,713 indexed pages: 10,132 have no picture and at least 200 characters; of those,
2,023 have more than 50 vector path objects (tables, charts). `auto` therefore embeds 8,604 page images instead of
16,713 (49% fewer; `../../eval/2026-10-20-indexing-choices.md`).

The page also says where the engine runs: "on the graphics card" or, when it fell back, "on the processor: pictures take
about 5 seconds each here, so Words only and file names are the quick choices". The engine reports its device already
(`/health`).

## How a change applies

The engine remembers, per source, the options of its last finished sync (a `source_options` table created with `IF NOT
EXISTS`: no schema version change, no forced re-index). A sync with different options re-processes every file of the
kinds whose option changed (PDFs, pictures or videos), even if the file itself did not change.

Re-processing must not mean re-embedding. New, general rule in the indexer: **a chunk whose id is already in the store
keeps its stored vector.** Chunk ids are content-derived (source, path, locator, content hash) and the embedding input is a
function of the same things, so the vector would come out the same. Going from `auto` to `all` embeds only the page images
that were skipped; going back deletes them; a PDF's text chunks are never embedded twice. The same rule makes edits
cheaper everywhere: appending to a long file embeds only the chunks that changed.

A file indexed by name and later by content must not keep its name vector under the same id: name chunks hash
`name:<file hash>` as their content, content chunks the file hash itself.

## Videos

- Extensions: `.mp4 .m4v .mov .mkv .webm .avi .wmv`. No size limit (the file is never read whole).
- File identity: SHA-256 of the size plus the first and last MiB, not of the whole file (hashing a 2 GB video to see
  whether it moved is not worth it). Change detection by size and modification time as for every file.
- `name`: one `video` chunk, its text the path, embedded as a document (title the file name).
- `content`: the `video` chunk plus up to 8 `video_frame` chunks, one at the middle of each eighth of the video, located by
  time (`{"t": 12.5}`), embedded as pictures (two at a time, as photos). A video that cannot be opened is recorded as a file
  error and keeps its name chunk.
- Decoding is PyAV (bundles ffmpeg's libraries; one 27 MB wheel, LGPL), part of the engine's locked dependencies: it comes
  with the engine install and the existing "Update" flow. No new download host.
- In the bar: a `video` kind with its own icon, a `vid:` / `video:` scope, and a frame match shows "at 1:23".

## Folders with nothing to index

OneDrive's backup moves Documents, Desktop and Pictures into the OneDrive folder and leaves the old folder with only links,
which the scanner never follows. On the owner's machine `Documents` and `Desktop` show "0 files searchable" with no reason.
The folder line now says why: "Nothing here AGI can read" and, when the folder is one of those Windows moved, "Windows keeps
your Documents in C:\Users\…\OneDrive\Documents" (from the known-folder API).

## Not in scope

- Speech in videos (needs a speech model), video thumbnails in the bar, per-folder options.
- A time estimate before indexing (it needs a pre-scan to count pages); the device sentence stands in for it.

## Tests and gates

- Engine: option changes re-process only the right kinds; reuse by id embeds nothing for an unchanged chunk; `auto` page
  rule on built PDFs (picture, scan, chart, plain text); video name and frames on a generated clip; a broken video keeps its
  name chunk.
- Launcher: config defaults and validation, options sent with every sync, a change of option starts a sync, `vid:` scope,
  frame locations.
- Golden eval with the defaults must pass the gate (page 9 of Week 10, the table, keeps its image under `auto`: it has
  almost no text).
- Measured on the real data: index time of the sample and of the PDFs, `auto` against `all`.
