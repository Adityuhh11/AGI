# Audio embedding spike (2026-10-28)

Question: can the audio encoder that is already inside the model files Omni installs make audio and the sound of videos
searchable, without a speech-to-text model? Method, numbers and limits. The scripts were throwaway (a Python script per
experiment, run through the engine's own `LlamaServer` and `LlamaEmbedder`, so the flags are the shipped ones).

Machine: RTX 4050 laptop (Vulkan), llama.cpp b11719, `embeddinggemma-2-Q8_0.gguf` + `mmproj-embeddinggemma-2-Q8_0.gguf`
(the files `internal/enginesetup/pins.go` pins, already on this machine).

## 1. The audio encoder is already installed

Reading the GGUF header of the projector we ship (`mmproj-embeddinggemma-2-Q8_0.gguf`, 554,821,024 bytes, SHA-256 pinned):

```
clip.has_audio_encoder = True      clip.audio.projector_type = gemma4a
clip.audio.block_count = 12        clip.audio.embedding_length = 1024     clip.audio.num_mel_bins = 128
tensors: 744 a.blk + audio conv/projection tensors, next to 208 v.blk (vision)
```

`llama-server` logs `init_audio: audio input is in experimental stage` and accepts `input_audio` content parts on
`/v1/embeddings` (`{"content": [{"type": "input_audio", "input_audio": {"data": <base64 wav>, "format": "wav"}}]}`);
`audio_url` is refused (400). Download for this feature: **0 bytes**. (ADR 0004 said the audio encoder was not part of the
shipped files; it is, and is unused.) Audio costs about 25 tokens a second; the server's physical batch of 1024 allows
about 40 s per request (a 65 s clip fails with "input (1632 tokens) is too large"), so windows are at most 30 s with the
flags we already run.

## 2. Quality

Corpus: spoken passages made with Windows' own speech synthesizer (clean, synthetic: the easy case), queries written in
plain words. "Transcript as text" is the best a perfect speech-to-text model could give: the exact transcript embedded as a
document, which is what a Whisper pipeline would index at best.

| test | audio vectors | transcript as text |
|---|---|---|
| 12 passages, 24 queries (top-1 / recall@3 / MRR) | 1.000 / 1.000 / 1.000 | 1.000 / 1.000 / 1.000 |
| cosine of the right clip (mean) / best wrong clip (mean of max) / off-corpus query (max) | 0.729 / 0.567 / 0.573 | 0.770 / 0.590 / 0.611 |
| 16 clips, 8 topics with 2 clips each, 3 voices (US male, US female, British female), query = a detail: top-1 | 1.000 (16/16; beats its same-topic twin 16/16) | 1.000 |
| 12 single-word queries (`Lisbon`, `Trajan`, `metronome`, `al dente`, `offside`, ...; `melatonin`, a word never said, still finds the sleep clip) | rank 1 for 12 of 12 | not run |

Speech mixed with real traffic ambience (`arunangshubanerjee-suburban-alley-traffic-ambience`), 12 clips per level:

| signal to noise | 20 dB | 10 dB | 5 dB | 0 dB | -5 dB |
|---|---|---|---|---|---|
| top-1 / MRR | 1.00 / 1.000 | 1.00 / 1.000 | 1.00 / 1.000 | 1.00 / 1.000 | 1.00 / 1.000 |
| cosine of the right clip | 0.739 | 0.734 | 0.726 | 0.711 | 0.679 |

Codec round trips (encoded with PyAV, decoded back, 16 kHz mono; cosine to the vector of the original WAV): mp3 0.9987,
flac 0.9996, m4a 0.9981, aac 0.9978, opus 0.9993. PyAV has decoders for vorbis, opus, wmav1, wmav2, wmapro, flac, mp3,
aac, alac and pcm (it has no vorbis or wma *encoder*, so those two were not round-tripped).

Real files from the owner's Downloads (first 30 s of each; the 12 spoken clips as distractors), queries plausible for each:
"traffic noise outside" -> the ambience recording (0.70); "people singing happy birthday" -> the three birthday videos
(0.69, 0.69, 0.69); "afrobeat drums" and "rap music with a beat" -> the afrobeat video (0.66, 0.67); "water flowing in a
river" -> the river video (0.64); "chocolate cake recipe" -> nothing above 0.56. **Miss:** "a tiger roars" ranked the river
video first (0.66) and the video named "Tiger Attack" not at all in the top 3: sound-event queries are weaker than speech
and music.

## 3. Windows

A 17 s passage moved into recordings of about 2 minutes made of other passages; the best window must overlap the target.
Easy version (36 recordings, hit = overlap of at least 3 s): 30/30, 30/15, 20/20 and 20/10 (window/stride, seconds) all
36 of 36; 10 s windows 35 of 36 (and 12.5 vectors per 3 minutes). That test cannot tell strides apart, so a strict one:
the passage starts at second 22, so a 30 s grid cuts it in two (12 passages):

| window / stride | best-window cosine, mean (min) | share of the passage inside the best window, mean (min) | vectors per minute |
|---|---|---|---|
| 30 / 30 | 0.656 (0.605) | 0.82 (0.69) | 1.2 |
| 30 / 20 | 0.673 (0.606) | 0.85 (0.77) | 1.9 |
| **30 / 15** | **0.709 (0.664)** | **1.00 (1.00)** | 2.3 |
| 30 / 10 | 0.694 (0.666) | 0.97 (0.83) | 3.0 |

Without overlap a passage cut by the grid scores 0.605 at worst, which is the level of a silent window (below) and of
nonsense; with a 15 s stride the worst case is 0.664 and the whole passage is always inside one window. 30/15 is the choice.
(The "vectors per minute" column counts the test recordings' own length, which includes a tail; 30/15 is 4 per minute in
steady state: 240 vectors, 0.7 MB, per hour of audio.)

Long audio, one 155 s recording of 8 passages with 4 s gaps, 30/15: each passage's best window overlapped it by 14 to 18 s
(8 of 8). A passage of 35 s as one request gave a vector at cosine 0.90 and 0.87 to the vectors of its two halves: a
long window blurs topics, which is why windows stay at 30 s.

## 4. Silence, noise and level

Cosine of a silent or near-silent 30 s window to the 12 spoken queries (max): digital silence 0.604, hiss at -60 dB 0.636,
room tone at -45 dB 0.614, against 0.57 for an off-corpus query on speech and 0.73 for the right clip. Windows with
nothing in them look like answers, so they must not be embedded.

Where to draw the line (12 clips scaled to a level, then gaussian noise of a level, 30 s each):

| RMS (dBFS) | 0.099 (-20) | 0.03 (-30) | 0.01 (-40) | 0.005 (-46) | 0.0025 (-52) | 0.001 (-60) |
|---|---|---|---|---|---|---|
| speech: top-1 / cosine of the right clip | 1.00 / 0.735 | 1.00 / 0.736 | 1.00 / 0.736 | 1.00 / 0.736 | 1.00 / 0.735 | 1.00 / 0.734 |

Noise at 0.0005, 0.001, 0.002, 0.003, 0.004, 0.006, 0.01 and 0.03 RMS: max cosine to the 12 queries 0.620, 0.627, 0.631,
0.626, 0.627, 0.622, 0.626, 0.624. The encoder ignores loudness, and noise at any level scores 0.62 to 0.63 (real
ambience scored 0.55 to 0.70 depending on the query). So loudness cannot tell quiet speech from noise, and a quiet recording
must not be dropped for being quiet: the gate removes only what has nothing in it, a loudest half second below RMS 0.002
(-54 dBFS). Noise above that is kept, because it is a sound someone may search for ("traffic"), and the golden eval with
audio in the index is where a noise floor that behaves like an answer would show.

## 5. Speed and memory

| | one 30 s window | notes |
|---|---|---|
| GPU, one request at a time | 0.13 to 0.21 s | 12 clips of 17 s in 2.6 s |
| GPU, two connections | 0.43 s for 6 clips (0.74 s one by one, 0.67 s as one request of six) | two in flight, as for pictures |
| CPU only | 3.75 s (8x real time per window; 4x with the 15 s stride) | a 30-minute recording: 119 windows, 7.4 min |

A 30-minute video (`Politics2024.mp4`, AAC 48 kHz stereo, 1,801 s): decode and resample 6.9 s, 119 windows embedded in 25.3 s
one at a time: 33 s in all (55x real time end to end, one request at a time). VRAM: the whole GPU rose from 2,151 to
2,206 MiB (+55 MiB) while embedding audio; the projector, audio encoder included, was already resident.

## 7. On the owner's real index (a copy of it, 41,866 chunks) with the media added

The real `Indexer`, `Searcher` and model, 38 files added (10 of the owner's videos and audio files, 28 spoken clips) with
`audio = content`: 194 chunks, 6 MB, no errors (a video without sound keeps its name only).

Audio sits on a scale of its own. Best cosine of nonsense against the audio windows only: p50 0.619, p90 0.647, max 0.665;
against everything else on the index p90 is 0.717. A right audio answer is 0.645 to 0.79 (median 0.73; 22 cases). With one
shared threshold that would hide real audio hits: 15 of 22 pass the 0.717 ceiling (spoken 15 of 18, **real sound queries 0 of
4**) and the rest would rank below text noise. How the ceiling depends on the number of windows (p90 of 60 probes, random
subsets): 5 windows 0.601, 10 0.611, 20 0.618, 40 0.627, 80 0.636, 156 0.647.

So `Searcher.calibrate()` measures the audio windows apart (from 10 windows; 0.60 below that) and a search adds the difference
to an audio window's cosine (`audio_offset`, +0.070 here), which puts its noise floor where every other kind's is. One
threshold and one ranking stay, and `confident` keeps its meaning. A covering index `chunks(kind, vec_row)` makes finding a
kind's rows cheap (it is `IF NOT EXISTS`: no schema change, no rebuild).

| | before the media | with the media |
|---|---|---|
| golden set (92 queries): top-1 / recall@3 / recall@10 / MRR | 0.273 / 0.377 / 0.429 / 0.329 | identical |
| golden false alarms | 0.039 | 0.039 |
| `search_files` latency, 60 queries (p50 / p95) | 31.0 / 55.0 ms | 31.3 / 56.0 ms |
| audio queries (22): right file first / first and confident | | 22 of 22 / 21 of 22 |
| golden missed nones | 0.267 (4 of 15) | 0.467 (7 of 15), explained below |

The golden set is graded on the owner's whole index here, not `Sample Data/` alone, which is why its absolute numbers are low
(77 answerable queries, many answered by other files); the point is that nothing moved. The three extra "missed nones" are
queries that my spoken corpus now answers: "guitar chords for a song" (best audio cosine 0.713, `guitar.wav`), "sourdough
starter feeding schedule" (0.694, `cook_a.wav`) and "symptoms of the flu" (0.668, `med_a.wav`). Where the line belongs, by
raw audio cosine: at 0.65, 21 of 22 correct hits are kept and 0 of 60 random probes and 3 of 77 answerable queries pass;
at 0.70, 18 of 22 are kept. The p90 calibration lands at 0.647, which is the right place; no tuning was needed. Re-check with
audio that has no answer in the golden set when the owner's own recordings are added.

Indexing speed was measured twice with different results, and the reason matters. First run (GPU otherwise idle): 30-minute
video 33 s, 133 to 212 ms a window. Last run: 876 ms a window (the 30-minute video in 116 s, 16x real time; 28 short clips in
12.7 s). In between, LM Studio had loaded a language model (its own `llama-server`, about 3 GB), so the 6 GB card held 5.2 GB
before the engine's own 1.1 GB: the card spilled into shared memory and audio embedding got six times slower. Decoding is
not the cost (7.5 s for the 30 minutes; WAV and base64 are 11 ms a window). The estimate and the File search page must say
that numbers assume the card is not shared with another large model; the processor rate (3.75 s a window) is the floor.

## 6. What this does not show

- All speech here is synthetic (or one noise type); the 30-minute video has no ground truth, so its hits (e.g. "immigration
  and the border" -> 14:30) are plausible, not verified. The build phase checks real recordings by ear.
- English only. Google's material does not state which languages the audio encoder covers. The owner's voice notes (WhatsApp)
  in other languages are the test; until then the setting says what it was measured on.
- No Intel or AMD GPU and no integrated GPU run.
- No Whisper run. Not needed to decide, but for the record the published sizes (whisper.cpp ggml, q5_1): tiny 31 MiB,
  base 57 MiB, small 181 MiB; q5_0 medium 514 MiB, large-v3 1.1 GiB; official runtime memory 390 MB, 500 MB, 1 GB for tiny,
  base and small.
- A third-party project (Embedding Gemma Transcribe, not Google's) trains a small reader on the same frozen audio encoder
  and reaches about 5% word error on English; it shows the encoder's frames carry word-level content, which fits the
  single-word results above. It is not a shipped model and not a dependency.
