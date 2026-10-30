# Indexing: scheduling a big index, and measuring a Mac

Status: sections 1 and 4 are built (the owner took the recommended answers: one daily window, offered in the guide for a first index over 2 hours, nothing indexed outside it). Sections 2 and 3 wait for a measurement on a Mac.

## 1. Time left (built)

The old estimate was `worked / done * (total - done)` over the whole job. Two things made it wrong:

- A change of speed in the middle (Settings > File search) was averaged with everything done at the old speed, so after
  Gentle to Fast the estimate stayed hours too long, and after Fast to Gentle it stayed too short.
- Files are processed newest first and differ a hundredfold in cost (a note against a scanned PDF), so the first files say
  little about the rest.

Now `indexing/eta.go` uses the rate of the last 90 s of work (not counting a pause), falls back to the average since the
speed last changed while that window is thin, and shows nothing for the first seconds after a change. It is rounded to the
minute as before. What it still cannot know is the cost of the files not yet reached; the pre-index figures in the guide
come from `internal/estimate`, which does know the kinds. Possible later step: weight `FilesDone` by that cost so the live
figure and the pre-index one agree. Not done, because the window already tracks the real rate and the weights would be one
more thing to keep right.

## 2. Is the speed setting broken on a Mac?

Read end to end, the path is the same on both systems: the page calls `SetPace`, the store notifies `main.go`, `Manager.SetPace`
calls the engine's `PUT /pace`, `Pacer.set_duty` wakes a rest already under way. Nothing in it names a system, and the
engine tests for it (`test_pace.py`) do not either. So a Mac-only failure is not in the code as read; what a Mac user can see
is real but common to both:

- The speed only rests the **embedding** calls (`PacedEmbedder`). PDF rendering, video and audio decoding and hashing run
  at full speed on the processor whatever the speed, so on a folder of videos "Gentle" still keeps a core busy.
- The time left was wrong after a change (section 1).

To settle it on a Mac: start an index, switch Fast to Gentle, and watch Activity Monitor's GPU history. The engine's access log is off, so
a request that never arrived would show only as an "indexing: telling the engine the pace" line in `omni.log`; if there is
none and the GPU stays flat out, the bug is real and is in the engine on that system.

## 3. Making indexing faster on a Mac (and elsewhere)

What is known: on Windows indexing is limited by the graphics card (text 53 chunks/s, pictures and pages 4.4 to 6.4/s,
`eval/2026-10-20-indexing-throughput.md`); server flags, batch sizes and request counts were measured and left alone. On a
Mac **nothing has been measured**: the engine runs `llama-server` with `-ngl 99` on Metal and the Windows batch sizes.

Proposed, in this order, each kept only if a measurement on the same files says so:

1. A script (`sidecar/spikes/`) that indexes `Sample Data/` and prints chunks per second by kind, the way the Windows run
   was done, so the owner can run it on the Mac and send the numbers. Nothing below starts before that.
2. On Metal: `-fa on`, `-np 1`, `-ub` 512 and 2048, `-t` (the performance cores only), and the number of pictures in flight
   (2 was the best on a discrete card; unified memory may want more).
3. Overlapping the processor's work with the card's: today a file is read, chunked and rendered, then embedded, one batch
   at a time. A worker that prepares the next batch while the card works on this one hides the preparation (PDF pages and
   video frames are the slow part on the processor). Worth doing on both systems if the profile shows the card idle between
   batches.

## 4. Scheduling a big index (decisions for the owner)

Wanted: someone with a very large folder can say "do this at night" instead of leaving the computer slow all day.

Smallest thing that is useful: **a daily window**, `[search] index_between = "22:00-07:00"` (empty by default = always).
Outside the window the manager holds the engine like a pause (the same mechanism as "wait for power": its own flag, so the
person's pause and the battery's are not disturbed), and the status says "Indexing waits for 22:00". New and changed files
inside the window are picked up as usual; searching is never held. The settings page gets two time fields and a checkbox
under the speeds. About 60 lines of Go in `indexing/` beside `pace.go`, and a test with the manager's injectable clock.

Questions:

1. Is a single daily window enough, or should it be per weekday? (Recommended: one window; weekdays when someone asks.)
2. Should a first index that the survey says will take more than some hours offer the window by itself in the guide?
   (Recommended: yes, one line on the last screen, off unless chosen.)
3. Outside the window, should a file the person just saved still be indexed at once? (Recommended: no, keep it simple; it
   is picked up when the window opens.)
