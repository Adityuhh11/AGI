# Audio search: what is said and played in audio files and videos

Date: 2026-10-28. Status: engine, launcher, interface and the in-bar offer are built and tested (steps 1 to 4 of
section 10); opening a match at its moment (step 5) and the owner's acceptance run on his own recordings are not done.
Building on `feat/audio-search`.
Evidence: `../../eval/2026-10-28-audio-embedding.md`. Decision record: `../../decisions/0007-audio-by-the-audio-encoder.md`.

## 1. The answer to "do we need Whisper?"

No. The 555 MB projector Omni already downloads (`mmproj-embeddinggemma-2-Q8_0.gguf`) contains EmbeddingGemma 2's audio
encoder (12 conformer blocks, 128 mel bins; read from the file's own header). The `llama-server` we already run embeds
audio through it into the same 768-d space as text, pictures and PDF pages. A text query finds a sound with no
transcript in between. DigUp does exactly this (it has no speech-to-text step either), and so does Google's own Video
Moments Finder demo.

Measured on this laptop (`../../eval/2026-10-28-audio-embedding.md`):

| | result |
|---|---|
| extra download, extra process, extra GPU memory | 0 bytes, none, +55 MiB while working |
| 12 spoken passages, 24 queries | top-1 24/24, **equal to embedding the perfect transcript as text** |
| same-topic clips told apart by a detail (16 clips, 3 voices) | 16/16 |
| single rare words (`Lisbon`, `Trajan`, `metronome`) | 12/12 |
| speech under real traffic noise | top-1 stays 12/12 down to -5 dB |
| music, ambience, singing (owner's files) | found; one sound-event query ("a tiger roars") missed |
| speed, graphics card | about 55x real time end to end (30 min of video in 33 s), 2 requests in flight faster still |
| speed, processor only | 4x real time (30 min in 7.4 min) |
| index size | 4 vectors a minute: 0.7 MB per hour of audio |

The "1 GB" in the brief is the large Whisper models; the quantized ones people use for search are 31 to 181 MiB
(tiny to small, whisper.cpp q5_1). The real cost of Whisper is not size: it is a second runtime and GPU resident set to
keep, its transcription errors on names and accents, and a feature (transcripts) whose only gain over the measured
approach is exact words and a snippet to show. We take the approach that is already measured to retrieve as well as a
perfect transcript, and keep the door open for a transcript tier (section 9).

What the approach cannot do, said plainly: it shows a time, not a quotation; it finds what a sound is *about*, so
"find the sentence where he says exactly this" is not a job for it; sound events (a tiger, a door slam) are weaker than
speech and music; and we have measured English only (section 8).

## 2. What the person sees

One choice, **Audio and sound**, with the same two answers as Videos:

| value | meaning |
|---|---|
| `name` | audio files are found by their file name (today's behaviour) |
| `content` | also by what is said and played: audio files, and the soundtrack of every video |

- **Config:** `[search] audio = "name" | "content"`. The default stays `name`: an index that exists changes only when its
  owner says so. Unknown values are an error like every other key.
- **Setup guide** (`lib/setup/Options.svelte`): a fourth card, "Audio and sound", beside PDF pages, Photos and Videos,
  with the cost the estimate gives for the person's own folders ("about 4 min for 38 hours of audio"). Once, before
  indexing starts, a computer with a graphics card gets `content` preselected (the cost is a minute an hour); a computer
  without one keeps `name` (4x real time is a choice to make, not a default), exactly as Videos is handled there.
- **Settings > File search > What to index:** the same card (`lib/settings/choices.ts`). A change on an existing index
  is the existing draft: "Save and re-index" says what it will cost; going to `name` removes the sound vectors and
  computes nothing.
- **In the bar, enabling it on the spot.** With `audio = name`, a file search (the Files mode, or any search that starts
  `aud:`, `audio:` or `sound:`) ends with one row after the files: "Also search what is said and played in your 212 audio
  and video files? About 6 min on this computer. Enter turns it on; Ctrl+Delete for not now." (`internal/soundoffer`,
  `palette/offer.go`, row kind `offer`). It is shown only when file search is on, the engine is ready and as new as the
  launcher (an older one would refuse the sync the choice starts, so the offer waits for the Update), the survey has counted
  audio or video in the folders, the query has three or more letters (or is a sound search), and the person has not said "not
  now" in the last 30 days (`state.json`, `waived`). Enter sets `audio = content` through the settings store, whose
  subscribers start the sync, and closes the bar; the tray and Settings show the progress. A new `aud:` scope (`audio:`,
  `sound:`) lists audio files, their sound and videos; `doc:` still finds audio by name.
- **Results.** An audio file is a row of kind `audio` (new icon, heading "Audio" in grouped results). Its best window
  shows as "at 1:15", the same as a video frame. A video's best match is a frame or a soundtrack window, whichever
  scores higher, and says which ("at 12:40, sound"). No snippet: there is no text.
- **Open at the moment.** Enter opens the file at the matched time when VLC, mpv or MPC-HC/MPC-BE is installed
  (`--start-time`, `--start`, `/start <ms>`), found from the registry or `PATH`; otherwise the default app opens the file
  from the start. The row says which will happen. (Last step of the build: it is the part that depends on the player.)

## 3. How it works in the engine

### 3.1 Windows

An audio track is cut into **30 s windows, a new one every 15 s** (measured, section 3 of the eval: 30/30 leaves speech cut
by the grid at cosine 0.605, the level of nonsense; 30/15 keeps the worst case at 0.664 and always holds the whole
passage). Windows start at 0, 15, 30, ... and the last one is kept only if more than 15 s of it is new, so every second is
in at least one window. A window is 16 kHz mono; the server resamples if not, but we decode and resample once with PyAV
(already an engine dependency, with decoders for mp3, aac/m4a, flac, opus/ogg, wma, alac, wav and every video container).

- Limits come from the server we run: a request must stay under the physical batch (1024 tokens, about 40 s at 25 tokens a
  second), so windows never exceed 30 s. No server flag changes, no engine restart.
- **Silence is never embedded.** Empty windows look like answers (cosine 0.60 to 0.64 against queries, above the 0.57 of an
  off-corpus query). A window is dropped when its loudest 0.5 s is below RMS 0.002 (-54 dBFS). Loudness cannot go higher:
  the encoder ignores it (speech at -60 dBFS ranks 12 of 12 with the same cosine as at -20) and noise of any level scores 0.62
  to 0.63, so a quiet recording must never be dropped for being quiet. A named constant with a test on generated silence,
  hiss, room tone and a very quiet tone. Noise above the line is kept (it is a sound someone may search for); the golden
  eval with audio in the index is where a noise floor that behaves like an answer would show.
- **Streaming.** Decoding is a generator holding one window and its overlap: a 3-hour podcast never sits in memory as a
  720 MB float array. Windows are embedded in groups of up to four, two requests in flight (measured 1.7x faster than one).
- A video with no audio stream (screen recordings, many renders) simply has no sound chunks; that is not an error.
- A file that cannot be decoded keeps its name chunk and is reported, like an unreadable video.

### 3.2 Chunks, routes, identity

| piece | change |
|---|---|
| `sources/rules.py` | `Route.AUDIO` for `.mp3 .wav .flac .m4a .aac .ogg .opus .wma` (taken out of the by-name list); no size limit (never read whole) |
| `types.py` | `ChunkKind.AUDIO` (an audio file found by its name, text = path, as `VIDEO` is for videos) and `ChunkKind.AUDIO_WINDOW` (30 s of sound, locator `{"t": 75.0}`, text empty, embedded as audio); `IndexOptions.audio` (`name`, `content`) |
| `chunking/audio.py` (new) | `AudioTrack` (opens a file or video, `windows()` generator, the silence gate), `audio_window_chunks`, the name chunk. The window chunk's content hash is `sha256(identity:audio1:<start>)`: the `audio1` is the geometry version, so a later change of window length or stride re-embeds instead of mixing |
| `chunking/video.py` | `video_identity` becomes `media_identity` (size, first and last MiB), used by both |
| `embedding/` | `Embedder.embed_audio(clips: Sequence[np.ndarray]) -> np.ndarray` (float32, 16 kHz mono, shape `(n, 768)`); `LlamaEmbedder` writes a 16-bit WAV with the standard library and posts it as `input_audio`, two in flight; `FakeEmbedder` (hash of the samples), `SerializedEmbedder` and `PacedEmbedder` pass it through (audio is paced like pictures: it is GPU work) |
| `indexer.py` | `_index_audio` and the soundtrack branch of `_index_video`; `_sound` is the streaming sibling of `_picture_vectors`: windows are decoded and embedded four at a time, and stored vectors and an identical file's (donor, by content hash) are reused for sound exactly as for frames; `_changed_routes` maps an `audio` change to `Route.AUDIO` and `Route.VIDEO` |
| `api/` | `IndexOptions.audio` in the request schema, `audio`, `audio_window` in the `kinds` values (`api.md`) |
| search | fusion is unchanged: audio windows have no keyword text, so they are ranked by cosine like images (invariant 10), after a shift onto the shared scale (section 4). `Searcher.calibrate()` measures audio windows apart; `FlatStore.search_vectors` takes per-kind offsets; `chunks(kind, vec_row)` is a covering index (`IF NOT EXISTS`, no schema change) |

Existing indexes: audio files were indexed as `file` chunks by name with a size-based identity. The identity changes, so
each audio file is processed again once, the old `file` chunk is replaced by an `audio` chunk, and it costs one short text
embedding per file. A video's soundtrack is added only when `audio = content`; a change of the `videos` choice reuses the
stored sound vectors (they are found by id; decoding is quick, embedding is what is skipped).

### 3.3 The cost, said before it is paid

`POST /survey` counts files and bytes by route without opening them, on purpose: a cloud-only OneDrive file would be fetched to
be counted (a test holds this). Duration is what audio costs, so the survey adds `seconds` to the `audio` and `video` routes,
guessed from each file's size and format (`sources/length.py`: mp3 20 KB/s, m4a and aac 16, ogg and opus 12, flac 100, wav 176,
video 300; the owner's files ran 16 to 32 KB/s for mp3 and 157 to 696 KB/s for video). It is a plan: the time left that indexing
reports once it runs corrects it. `internal/estimate` counts one window per 15 s of sound (at least one per audio file), at 0.28 s
on the card and 3.8 s on the processor (30 minutes of video in 33 s, measured with the card to itself). A video with no sound is
counted too, so the estimate is a ceiling. Sharing the card with another large model made it six times slower
(eval section 7); the estimate cannot see that.
The first draft of this spec had the survey open the headers of 40 files; the cloud-file rule above is why it does not.

## 4. Search quality is gated, not assumed

Invariant 7: retrieval quality is gated. Audio is a new kind of chunk in the same index, so two things are measured before
it ships:

1. **Audio queries are added to the golden set**: the spoken passages (generated with the Windows synthesizer, 24 queries),
   and the owner's own voice notes and videos (about 10, checked by ear). Gate: audio top-1 and recall@3 recorded in
   `baseline.json`, and the existing text and picture metrics unchanged (more than 0.02 down fails).
2. **Audio must not push existing answers down**, and nonsense must still be "not confident". Measured (eval section 7):
   audio windows sit on a lower scale (nonsense p90 0.647 against 0.717 for the rest; right answers 0.65 to 0.79), so one
   shared threshold hid every real sound query (0 of 4) and ranked audio under text noise. The fix is built: audio is
   calibrated apart (`search/calibration.py`, from 10 windows, 0.60 below) and shifted by the difference
   (`Searcher.audio_offset`), so one threshold and one ranking still hold. On a copy of the owner's real index the golden
   metrics are identical with the media added, search latency is unchanged (31.0 to 31.3 ms p50), and 21 of 22 audio queries
   are first and confident.

Sample audio goes in `Sample Data/` (git-ignored, like the rest); CI keeps using the fake embedder for logic and the real
model only under the `model` marker.

## 5. Constraints checked

- **Hotkey path:** untouched; nothing here runs when the bar opens. The in-bar row is a palette item built in-process.
- **Models only on the GPU:** the audio encoder is already resident in the llama-server; index and search stay on the CPU.
- **6 GB VRAM:** +55 MiB measured while embedding audio.
- **Privacy:** no new network call and no new download; audio never leaves the machine.
- **Install and update:** nothing new to download; every engine already has the projector and PyAV. The engine's source
  changed, so every installed engine shows the existing Update button once; until it is pressed the launcher sends no `audio`
  option and makes no offer.
- **Pacing and battery:** audio goes through `PacedEmbedder` (speed and pause) and the battery wait like any indexing.

## 6. Not in scope

- Speech-to-text: no transcripts, no quotation, no exact-phrase search, no dictation. (Section 9.)
- Speaker identification, per-speaker search, language detection, live capture of the microphone.
- Lyrics, music identification ("which song is this").
- Audio in other containers' attachments (Office files, PDFs).
- Per-folder audio settings.

## 7. Tests and gates

- **Engine, fake embedder, generated audio (no model):** windows of a 70 s clip start at 0, 15, 30, 45 and the last is kept
  only with more than 15 s new; the silence gate on generated silence, hiss, room tone and a tone with speech-level energy;
  a video with no audio stream; a file that cannot be decoded keeps its name and is reported; an unchanged file embeds
  nothing; a moved or copied file takes the vectors of the identical one; changing `audio` re-processes only audio and
  video; going to `name` removes the sound chunks and embeds nothing; ids change with the geometry version.
- **Real model (`model` marker):** a generated spoken clip is found by a query about it; the vector equals the one from a
  direct server call; a 30 s window is accepted and a 41 s one is the server's error, caught by never sending it.
- **Launcher:** config default and validation, the option sent with every sync, estimates for audio, `aud:` scope, the
  "at 1:15" location for audio windows, the enable row (shown and hidden by each of its conditions, Enter turns the option on,
  "Not now" is remembered), player detection and the start-time arguments (the process start is injected).
- **Frontend:** the card in setup and in settings, the plan sentence for an `audio` change, the Audio group and icon.
- **Measured on real data before it is called done:** the owner's audio and videos indexed with `content`, time and index
  size against the estimate, ten queries judged by the owner's ear, and the golden eval with and without audio.

## 8. Risks and what we do about them

| risk | handling |
|---|---|
| the audio encoder's languages are not documented; we measured English | test the owner's non-English voice notes before release; the choice's text says "measured on English" until then; the remedy for a weak language is a transcript tier, not hiding the problem |
| synthetic speech is easier than real rooms | the noise test and the real files say it holds, but the acceptance run is the owner's recordings |
| audio support in llama.cpp is marked experimental | the pinned build is what was measured; an engine update re-runs the audio model test before the pin moves |
| integrated or non-NVIDIA GPUs not measured | the survey/estimate uses the processor rate when the engine runs on the processor; Intel and AMD get measured when a machine is available (`STATUS.md` S6) |
| another large model holds the graphics card (LM Studio with a 3 GB model: 876 ms a window instead of 133 ms) | measured, not fixable from here; the cost sentence says it assumes the card is not shared, and the processor rate is the floor |
| a long video makes indexing look stuck | progress is per file today; windows of one very long file report as it goes (the manager already reports files, so the cost sentence names the longest file) |
| sound-event queries rank weaker | documented in the choice's text and the README; not worked around |

## 9. Later, not now: a transcript tier

If speech-to-text for dictation is built, it has its own spec (model choice, hotkey, injecting text at the cursor).
Its transcripts could then be indexed as `transcript` chunks next to the sound windows: exact words, quotation snippets,
and a second path for languages the audio encoder is weak in. This spec leaves nothing in its way: chunk kinds are strings,
locators carry a time, and ranking is by one comparable score. It does not need Whisper to ship audio search, and shipping
audio search does not commit us to any speech model.

## 10. Build order

1. Engine core: `embed_audio` and its wrappers, `chunking/audio.py`, route and kinds, `IndexOptions.audio`, indexer, API, tests.
2. Real-model checks and the golden eval with audio; record the baseline.
3. Launcher: config, `sidecar` options, search kinds and locations, scope, estimate and survey.
4. Frontend: the card in setup and settings, the Audio icon and group, the in-bar enable row.
5. Open at the moment in the player (not built).
6. Docs in the same commits (`ARCHITECTURE.md`, `api.md`, `STATUS.md`, README privacy and cost sentences); the owner's
   acceptance run on his own audio.
