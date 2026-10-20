# Indexing throughput and search overlap (2026-10-20)

Where indexing time goes, and the changes that came out of it. RTX 4050 Laptop, llama.cpp b11719 Vulkan (the installed
engine), real model; the launcher's own engine was running beside it, so absolute numbers carry its noise.
Scripts: a scratch profiler around `Indexer.sync` and the embedder (not kept).

## Where the time goes

The real index (46,392 chunks after the owner's folder changes; 56,252 at its largest): 16,713 PDF pages and 997 images
against 28,678 text chunks. Measured rates: text about 53 chunks/s, images and pages about 4.4/s, so pages and images are
about 85% of a full index (some 70 of the 80 minutes), text about 10 minutes.

`Sample Data/` (96 files, 431 chunks), one sync, before any change: 24.6 s, of which image embedding 11.4 s (46 images
and pages; PNG encoding alone 2.4 s), text 9.0 s, writing to the store 0.7 s.

## What was tried

| Experiment | Result | Kept |
|---|---|---|
| Text: request size 4, 8, 16, 32 | 52.8, 54.3, 55.1, 43.1 chunks/s | no: the GPU is the limit |
| Text: 2 or 4 requests in flight | 48.5, 50.9 chunks/s | no |
| Server flags `-fa on`, `-np 1`, `-ub 2048`, `-c 4096` | within run-to-run noise (46 to 54 chunks/s) | no |
| Images: 1, 2, 4 requests in flight (one image each) | 4.38, 6.38, 6.47 images/s, vectors identical | 2 |
| Image upload as PNG default, PNG level 1, PNG level 0, BMP | encode 83, 38, 41, 13 ms; encode + embed 320, 281, 325, 280 ms; same pixels, cosine 1.000000 | PNG level 1 |
| Images per `SerializedEmbedder` slice: 2, 4, 8 | image time 8.45, 7.83, 7.86 s on the sample | 2 (a query waits for about one image, not four) |
| llama-server memory: `-c`, `-np`, `-cram 0`, `--no-cache-prompt`, `--no-host`, `--no-repack`, `-lm none` | private bytes 1,757 to 1,895 MB in every case, resident about 60 MB | no: the private bytes are the driver's commit for the VRAM, not RAM in use |

## Result

Images go two at a time on connections of their own, encoded as PNG level 1, and image files are batched across files as
text already was. `Sample Data/`, three runs each, old code against new: 19.1 / 17.3 / 17.3 s against 15.9 / 15.8 /
16.2 s (images 9.6 to 10.2 s against 8.4 to 8.6 s). Golden eval unchanged: top1 0.896, recall@3 1.000, recall@10 1.000,
MRR 0.946, false alarms 0.000, missed nones 0.000.

## Search

On a copy of the real index (98 queries, three rounds): query embedding p50 10 to 14 ms, vector scan 4.4 ms, keyword
search 4.8 to 5.9 ms (p95 19 ms), path search and chunk lookup 0.35 ms each. Keyword and path search now run on a thread
of their own while the query is embedded. `search_files` p50 22.7 ms before, 15.7 ms after in the cleaner of two runs
(the other: 21.1 ms, with the launcher's server busy on the same GPU).

## Not done, and why

- Store writes (0.4 to 0.7 s per sample sync, about 4 ms a file): too small to batch.
- Thumbnails through the Windows shell: 262 ms for a 12 MP JPEG on first use against 300 ms in Go, 9 ms only when Explorer
  has the thumbnail cached; not worth the change for speed.
- 10,358 of the 16,713 indexed PDF pages have 200 characters of text or more and no image object. Their page vectors cost
  30 to 40 minutes of a full index; whether they find anything their text chunk does not is a product question, not
  measured by the golden set (it scores files, and a file's text chunk finds it).
