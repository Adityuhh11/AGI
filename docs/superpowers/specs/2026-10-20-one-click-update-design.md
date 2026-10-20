# One-click update

Date: 2026-10-20. Status: requested by the owner on 2026-10-20 ("check for updates and then update it directly").
Builds on `docs/decisions/0005-update-check.md`, which built the check and left installing for later. Branch `feat/self-update`.

## Goal

Settings > About says a newer version exists, and one button, "Update to X", downloads it, checks it, replaces the program and
starts the new one. Nothing happens until the button is pressed, and nothing is touched until the download has been verified.

## Decisions

| # | Decision | Why |
|---|---|---|
| 1 | **Press to update, never automatic.** The daily check stays optional and only tells | The privacy rule makes a download a thing the person starts; a silent self-replacing program is also the riskier one |
| 2 | **The program is replaced where it is.** No move to a fixed folder | It works from wherever the zip was unpacked, start at login keeps pointing at the same path, and a move would be machinery for a problem nobody has. If the folder cannot be written (Program Files) the page says so and offers the release page |
| 3 | **Verified against GitHub's own SHA-256 of the file** (`digest` in the release's asset list, checked on the real v0.3.0) | Nothing extra to publish or maintain. No digest, or a different one: nothing is installed and the release page is offered |
| 4 | **Only `agi.exe` is taken from the zip**, by its exact name | The zip is ours and holds `agi.exe` and a README; taking one named entry means no path in a zip can write anywhere else |
| 5 | **Downloads only from this project's releases** (`https://github.com/Adityuhh11/AGI/releases/download/`) | The address comes from the check, never from the page, and is refused if it points anywhere else |
| 6 | **Test builds named by date and commit are offered the update; a developer's `dev` build never is** | As `update.Newer` already decides: the friend's first build should be able to move up, the owner's own builds must not be overwritten |

## How it works

1. **Check** (exists): `update.Checker` now also keeps the release's zip: name, address, size, SHA-256.
2. **Download** to `%LOCALAPPDATA%\AGI\update\`, with progress, cancellable; at most 200 MB.
3. **Verify** the whole file against the SHA-256. A mismatch deletes it and stops.
4. **Extract** `agi.exe` from the zip into the program's own folder as `agi.exe.new` (same drive, so the next step is a rename).
5. **Swap**: Windows lets a running program be renamed but not deleted, so `agi.exe` becomes `agi.exe.old` and `agi.exe.new` becomes
   `agi.exe`. If the second rename fails the first is undone. Until this step nothing on disk has changed.
6. **Restart**: the running launcher starts the new `agi.exe` with `--wait-for <its own process id>`, then quits. The new one
   waits for that process to be gone (at most 15 s) before it starts, because a second launcher while the first still runs would
   only hand over to it and exit; then it deletes `agi.exe.old` and carries on as usual (engine restarts with it).

**The job object.** The launcher puts itself in a job that kills everything in it when it ends (that is how the search engine
dies with the launcher, even if the launcher is killed). A process started the ordinary way would be in that job and die with the
old launcher. So the job gets `BREAKAWAY_OK` and the restarted launcher is created with `CREATE_BREAKAWAY_FROM_JOB`; the engine,
started the ordinary way, still dies with its launcher.

**What the check protects against, and what it does not.** Corruption, a truncated download and a tampered CDN copy: the SHA-256
comes from the API over TLS, the file from another address. Not a compromised GitHub account or repository: whoever can publish a
release can publish its digest too. Code signing (`docs/decisions/0005`'s "later") is what would close that, and is not here.

## What goes wrong, and what the person sees

| Failure | Result |
|---|---|
| No network, GitHub refuses, download cut | an error in the About page; nothing on disk changed; the button stays |
| Digest missing or not matching | "could not be verified", nothing installed; the release page is offered |
| Folder not writable | "cannot write next to agi.exe: <reason>"; the release page is offered |
| Antivirus holds the new file | the swap's error is shown; the first rename is undone |
| Second rename fails after the first | the first is undone, the old program is still in place |
| New program does not start | the old file is in `agi.exe.old` and the page of the release says how to get it back (rename it); not automated |
| Pressing the button twice | the second press is refused while one runs |

## Where it lives

| Package | Job |
|---|---|
| `internal/update` | `Release` keeps the zip's asset; `Install` (download, verify, extract, swap) is plain Go and tested against a local server and temporary folders; `restart_windows.go` starts the new program and waits for the old one |
| `internal/sidecar` | the job gains `BREAKAWAY_OK` |
| `internal/settingsui` | `StartUpdate`, `CancelUpdate` and `UpdateProgress` for the page |
| `cmd/agi` | `--wait-for`, the cleanup of `agi.exe.old`, and the restart function the service calls |
| Frontend | the About page: "Update to X", progress, errors, the release-page fallback |

## Order of work and acceptance

| Step | Content | Done when |
|---|---|---|
| U1 | asset and digest in the check | table tests on real-shaped API answers (with and without a digest) |
| U2 | `Install` | tests: success, wrong digest, truncated file, a zip with the wrong entries or a path in it, unwritable target, rollback after a failed second rename, cancel |
| U3 | job breakaway, restart, `--wait-for`, `.old` cleanup | a restarted launcher outlives the one that started it |
| U4 | service and page | pressing the button twice is refused; progress reaches the page |
| U5 | end to end against the real v0.3.0 | a build stamped 0.0.1, run from a scratch folder in a clean profile, offers 0.3.0, installs it, restarts as 0.3.0 (About says so), leaves no `.old` and exactly one launcher |

## Risks and open questions

- **The restart is the least certain part** (a job, a process id, a single-instance lock). U3 is tested on the real thing before the page is built on it.
- **Antivirus** may slow or hold a freshly written program. The swap retries a few times before it gives up.
- A broken release cannot be undone by the program; the previous file is kept as `agi.exe.old` until the next start.
