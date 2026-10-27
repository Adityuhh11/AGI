# 0005: Update check, opt-in; installing is a button the person presses

Status: accepted, 2026-10-19. Amended 2026-10-20: the program can now install the update when its person presses the button
(see the end of this note and `superpowers/specs/2026-10-20-one-click-update-design.md`).

## Context

The launcher is shared as a zip. Until now the only way to learn of a new version was to be told, and
a person who is told replaces `omni.exe` by hand. The project rule is that nothing leaves the machine
except calls to a configured LLM provider, and that there is no telemetry.

## Decision

- **Ask.** (Installing was added on 2026-10-20, below.) `internal/update` asks `api.github.com/repos/Adityuhh11/Omni/releases/latest`
  for the newest release and compares its tag with the build's version. When it is newer, Settings >
  About says so and opens the release page; the tray shows "Update available". The person downloads and
  replaces the file.
- **Off until asked.** "Check for updates" in About is a button. "Check once a day" is a setting that is
  off by default (`check_updates` in `config.toml`); turned on, the first check runs at once, then a
  minute after each start and daily. The request has a `User-Agent` (GitHub requires one) and carries
  nothing about the person or the machine. AGENTS.md's privacy rule names this exception.
- **Version numbers.** A release is tagged `vX.Y.Z` and `scripts/release.ps1 -Version X.Y.Z` stamps the
  same number into the exe. A test build keeps the date-and-commit name; it counts as older than any
  release, while `dev` (a developer's build) is never offered an update. `/releases/latest` skips
  pre-releases, so a tester's pre-release never reaches anyone as an update.

## Alternatives

- **A check on by default.** Rejected: it is a request to a server the person did not name, and the
  privacy rule is the product's promise. A person who wants it turns it on in one click.
- **Replacing the exe itself** (download the zip, check its SHA-256, rename the running exe to `.old`,
  write the new one, restart). Deferred, not rejected: it is worth building when releases come often.
  It needs the exe in a fixed place (not `Downloads`) and a pinned hash per release; a file the program
  downloads itself carries no "from the internet" mark, so it would also skip SmartScreen.
- **An installer or MSIX** that updates through Windows. Rejected for now: signing and a store or
  installer toolchain cost more than the zip needs.

## Consequences

- The first build with the check is the last one that has to be replaced by hand; builds before it
  (`v0.1.0-friend1`) cannot learn of anything.
- GitHub allows 60 anonymous requests an hour per address; one a day is far below that.
- Moving `omni.exe` breaks nothing here, but "Start when I sign in" remembers the path (see
  `internal/autostart`) and must be switched off and on again.

## Amendment, 2026-10-20: one-click install

The person asked for the update to be installable from About. Built as the spec says: **a button the person presses, never
automatic**. "Update to X" downloads the release's zip from this project's releases only, checks it against the SHA-256 GitHub
publishes for the file (nothing is installed without one), takes the file named `omni.exe` out of it, renames the running
program to `omni.exe.old`, puts the new one in its place (undoing the first rename if the second fails), and starts it with
`--wait-for <pid>`; it waits for the old launcher to end (a second launcher while the first runs would only hand over and exit),
deletes `omni.exe.old` and opens Settings on About. The program is replaced where it is: no fixed install folder, so start at
login keeps working and the folder only has to be writable (checked before the download). The launcher's kill-on-close job now
allows a deliberate breakaway so the successor is not killed with it; the search engine, started the ordinary way, still is.

What the checksum does and does not protect: a corrupt, truncated or tampered-with download, yes; a compromised GitHub account
or repository, no (whoever can publish a release can publish its digest). Code signing would close that and is not done.

A release older than this feature would not know `--wait-for`, lose the race for the single-instance lock and leave no launcher
running; it is never offered, because only a newer release is, and every release from this one on has the feature. The
first update that really crosses from a build with the installer to one with it is therefore the next release after this.
