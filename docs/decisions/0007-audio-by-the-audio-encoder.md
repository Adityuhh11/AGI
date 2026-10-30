# 0007: Audio is embedded by the model's own audio encoder, not transcribed

Date: 2026-10-28. Status: accepted. Numbers: `eval/2026-10-28-audio-embedding.md`.

## Decision

Audio files and the soundtracks of videos are searched by embedding 30 s windows (a new one every 15 s) with the audio
encoder inside `mmproj-embeddinggemma-2-Q8_0.gguf`, through the `llama-server` the engine already runs. A text query lands
in the same 768-d space as every other kind of chunk. No speech-to-text model, no extra download, no new process.

## Why

- The encoder is already on every machine that has file search (the projector has been 555 MB since ADR 0004; its audio half
  was never used). ADR 0004's line "the audio encoder is not part of the shipped files" was wrong.
- Measured, it retrieves as well as the best a transcript could: 24 of 24 top-1 on spoken passages, equal to embedding the
  perfect transcript as text; 16 of 16 on same-topic pairs told apart by detail; single rare words found; speech held at
  -5 dB of real noise; it also finds music and ambience, which a transcript cannot.
- It is cheap: about 55x real time end to end on the GPU, +55 MiB of video memory, 4x real time on a processor alone.
- It is the approach DigUp takes and the one Google's own demo (Video Moments Finder) takes.

## Alternatives

| alternative | outcome |
|---|---|
| whisper.cpp (tiny, base or small, 31 to 181 MiB quantized) then embed the text | a second model, a second runtime and its GPU memory, and its mistakes (names, accents, noise) become ours. Gains exact words and a snippet to show. Not needed for the measured quality |
| a bigger Whisper (medium 514 MiB, large 1.1 GiB) | the "1 GB overhead" that was feared; no reason to pay it for search |
| Moonshine (27M parameters, English only) | smaller than Whisper, still a second model, English only |
| audio encoder, one window per file (up to 5.5 minutes in one request) | blurs topics (a 35 s clip sits at cosine 0.87 to 0.90 from each half) and needs a bigger server batch |
| 30 s windows with no overlap | a passage cut by the grid scores 0.605 at worst, at the level of silence; 15 s overlap raises the worst case to 0.664 |

## Consequences

- Hits carry a time, not words: the bar shows "at 1:15", never a quotation. Exact-phrase search ("he said ...") is not what
  this does; it finds what the sound is about.
- Empty windows score like answers (0.60 to 0.64) and are never embedded (RMS 0.002 gate, tested); the encoder ignores loudness, so
  quiet recordings are searched like loud ones, and noise at any level scores 0.62 to 0.63.
- Sound events are weaker than speech and music ("a tiger roars" missed on the owner's files).
- Languages other than English are unmeasured. If they prove weak, the answer is a transcript tier (below), not a replacement.
- Speech-to-text for dictation anywhere is a different feature with its own model and its own spec. If it ships, its
  transcripts can be added to this index as `transcript` chunks (exact words and snippets) without changing anything here.
