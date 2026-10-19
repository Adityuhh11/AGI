# 0005: Update check, opt-in, no self-install

Status: accepted, 2026-10-19.

## Context

The launcher is shared as a zip. Until now the only way to learn of a new version was to be told, and
a person who is told replaces `agi.exe` by hand. The project rule is that nothing leaves the machine
except calls to a configured LLM provider, and that there is no telemetry.

## Decision

- **Ask, never install.** `internal/update` asks `api.github.com/repos/Adityuhh11/AGI/releases/latest`
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
- Moving `agi.exe` breaks nothing here, but "Start when I sign in" remembers the path (see
  `internal/autostart`) and must be switched off and on again.
