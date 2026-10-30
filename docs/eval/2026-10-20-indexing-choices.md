# Indexing choices: measurements (2026-10-20)

RTX 4050 Laptop, llama.cpp b11719 Vulkan, real model,
a fresh index for every run; the launcher's own engine was running beside it.

## What a machine without a graphics card faces

The same laptop, the same model, forced off the discrete card:

| | RTX 4050 (Vulkan) | Intel UHD (Vulkan) | CPU only |
|---|---|---|---|
| one picture or PDF page | about 0.16 s | 4.78 s | 4.87 s |
| text | about 53 chunks/s | 2.6 chunks/s | 1.4 chunks/s |
| one query | 6 to 14 ms | 72 ms | not measured |

The engine skips integrated Intel GPUs and runs on the CPU instead; on this UHD that costs nothing for pictures and half the
text speed. A stronger integrated GPU (Iris Xe, Radeon) is untested.

## The `auto` page rule on the owner's index

16,713 indexed PDF pages in 533 PDFs. 10,132 have no picture and at least 200 characters of text; of those, the number of
vector path objects (rules, boxes, table lines, drawn charts):

| path objects | 0 | 1-20 | 21-50 | 51-100 | 101-300 | over 300 |
|---|---|---|---|---|---|---|
| pages | 1,756 | 5,542 | 811 | 739 | 957 | 327 |

Pages with more than 50 are kept as pictures (tables, charts, diagrams). `auto` therefore skips 8,109 pages and embeds
8,604 (49% fewer page vectors): 21 to 31 minutes less of a full index on this GPU (at 6.4 to 4.4 pictures a second) and
about 11 hours less on a CPU.

## Index time per choice

| folder | pdf_pages | photos | videos | time | chunks |
|---|---|---|---|---|---|
| `Sample Data/` (96 files) | all | content | name | 18.2 s | 20 page pictures, 26 images, 405 text |
| `Sample Data/` | auto | content | name | 16.5 s | 11 page pictures, 26 images, 405 text |
| `Sample Data/` | text | name | name | 8.0 s | no pictures; images by name |
| 9 real videos (5 s to 62 s) | | | name | 0.3 s | 9 names |
| 9 real videos | | | content | 22.8 s | 9 names, 72 frames |

With `videos = content`, "red ball floating in a river", "hello kitty singing" and "four spheres animation" each put the
right video first; for the second the best chunk was a frame (`at 0:07`). Frames are taken by seeking: a 30-minute video
decodes its eight frames in 0.44 s, a 5-second one in 0.26 s (spike, PyAV 19.0.1).

## Quality

Golden eval with the defaults (`auto`, `content`, `name`): top1 0.896, recall@3 1.000, recall@10 1.000, MRR 0.946, false
alarms 0.000, missed nones 0.000: identical to the baseline. Page 9 of Week 10 (the t table, almost no text) keeps its
picture under `auto`.

## Switching

A change of choice re-processes only that kind of file and embeds only chunks that did not exist (`test_sample_data.py`,
`test_indexer.py`): `auto` to `all` embeds the skipped pages, `all` to `text` deletes every page picture and embeds
nothing. An index built before the choices existed is treated as `pdf_pages = all`, so the first sync under the default
`auto` drops the plain pages' pictures without computing anything.
