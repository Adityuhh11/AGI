# One-click update, tested (2026-10-20)

Everything below was
run on the real binary on this laptop, in a clean profile (`APPDATA` and `LOCALAPPDATA` in a scratch folder), from a folder with
spaces in its name where noted, with the owner's own launcher stepped aside by process id and brought back.

## Unit and integration tests (`go test ./...`, also under the race detector for `internal/update`)

- **Install** (`internal/update/install_test.go`, a local server and temporary folders): success; the old program kept as
  `.old`; progress bytes in order; a wrong checksum; a truncated download; a download of the wrong size; bigger than the limit;
  an address outside the project's releases (never contacted, three variants); a server error; a redirect; zips with no
  `omni.exe`, a nested one, a path out (`../omni.exe`), another case, two, a directory, not a program, empty, not a zip, empty zip
  (each leaves the folder and its neighbours untouched), and an extra entry with a path out that is ignored; a folder that cannot
  be written (found before the download); a rename that fails (the old program back in place; moved aside failing; even the
  put-back failing, where the error names the file); a rename that fails twice and then works; leftovers of an earlier update; a
  release with nothing to verify; cancel; a second install refused; done called once.
- **The check** (`update_test.go`): the zip and its digest from a real-shaped answer; no digest, another algorithm, too short, not
  hex, no zip: none of them is installable.
- **The restart** (`restart_windows_test.go`, a test program that plays both launchers): a successor created with the breakaway
  flag outlives its parent inside the launcher's real kill-on-close job; the control, an ordinary child, dies with it (so the test
  would notice); `WaitForExit` on an ended, a running, a dying and a nonexistent process; `--wait-for` parsing; `.old` removal.
- **The service** (`settingsui`): refuses without a found release or a checkable zip, passes the program's path, restarts only
  after a successful install, tells the page when the restart could not happen, forwards progress and cancel, and says whether a
  release can be installed.

## End to end on the real launcher

**Against the real v0.3.0 on GitHub** (this branch's code stamped 0.0.1, in a scratch folder): About offered 0.3.0; the program
that was installed is byte-identical (SHA-256) to the `omni.exe` in the published zip; the 0.0.1 build was kept as `omni.exe.old`; the
config and the pins were untouched; the old launcher exited and a new one started. **That run found a limit of the test, not of the
feature**: v0.3.0 predates `--wait-for`, so the successor started at once, found the old launcher still holding the single-instance
lock, handed over and exited, leaving no launcher running. It cannot happen to a real update (only a newer release is offered, and
it has the feature; see the amendment to ADR 0005), but it is why the next runs use a local release.

**Against a local release** (a local server giving GitHub's answer and a zip holding a 9.9.9 build that has the feature; the old
build has the two addresses set at build time, which is the only difference from a release build):

| What | Seen |
|---|---|
| About after the check | "Version 9.9.9 is available", "Update to 9.9.9", "Open the release page" |
| While it downloads | "Downloading 0.1 of 5.4 MB", a progress bar, Cancel |
| After the update | old launcher gone in 2.9 s; the new one started and logged "started as the new launcher"; exactly one launcher running; the program on disk byte-identical to the 9.9.9 build; the folder holds `omni.exe` only (`.old` deleted by the new launcher); the config and the pins as they were; Settings opened by the new launcher on About, saying "Omni 9.9.9" |
| Same, in a folder with spaces in its name | the same |
| A wrong checksum | "The download does not match the checksum GitHub published for it, so nothing was installed. You can try again, or download it from the release page."; launcher alive, program and folder untouched, no download left behind |
| A release with no checksum | no "Update to" button; "Open the release page" and "This release has no download that can be checked, so it cannot be installed from here." |
| Cancel while downloading | "The update was cancelled."; nothing changed, no download left behind |
| A folder the user cannot write to (an access-control deny on the folder, path with spaces) | "Cannot write next to omni.exe: ... Access is denied." before anything is downloaded; nothing changed |

## Not covered

- A virus scanner holding the new file (the swap retries five times, 200 ms apart; unit-tested with a failing rename, not with a real scanner).
- A program in `Program Files` (the access-control test stands in for it).
- An update across two releases that both have the installer: that happens for the first time with the release after this one.
- A new program that starts and then misbehaves: the previous one is kept as `omni.exe.old` until the new one has started once, and putting it back is by hand.

## Opening speed

`cmd/panelbench -real` is unaffected by this work (the bar's page and panel did not change): `dev` and this branch measured side by side,
twice each, while the owner's own launcher was syncing its index (a `llama-server` and the Python engine busy): painted p50 72.5 and
72.6 ms for `dev`, 66.7 and 66.4 ms for this branch (p95 85 to 102 ms for both). The 28 ms p50 of the pins and clipboard measurements was on an
idle machine. So **while an index sync is running the bar opens in about 70 ms at p50 and sometimes just over 100 ms at p95**: the
GPU the page is drawn on is shared with the engine. That is not new and not caused by the update; it is worth keeping in mind next to the
"opens in under 100 ms" rule, and the indexing speeds in Settings > File search are the lever.
