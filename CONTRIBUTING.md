# Contributing to AGI

AGI is a small project with one maintainer and a plan for where it is going. Pull requests are not open yet (the project is still being
made ready for other people); this file says how they will work, and issues are welcome now.

## Open an issue first

For a fix as much as for a feature. An issue gets you a clear "yes", "not now" or "here is how it should fit" before you spend an evening on
it. A pull request that arrives without one may be closed with a link back here.

## Pull requests

Once an issue says go:

- Branch from `dev` and open the pull request into `dev`. `main` only changes by a pull request from `dev` that the maintainer merges.
- One fix or one feature, with the reason in the description. Conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`, `chore:`), small and focused.
- CI runs on every pull request: `go vet`, `go test`, the page's tests and type check, `ruff`, `pyright` and `pytest`. Run them first (README, "Build from source").
- Update the docs that describe the code in the same commit (`docs/README.md` says which ones).
- If it changes what search returns, run the eval (`uv run python -m agi_memory.eval` in `sidecar/`) before and after and include both results.
  For speed or memory changes, measure before and after on the same computer. Choices that affect speed, memory or quality are decided by a
  measurement, not a guess.
- Nothing that sends data elsewhere (telemetry, cloud services) fits: file search runs on the person's computer, and the only network calls are the ones
  listed in the README's privacy table.

## How the code is kept

The rules are in [`AGENTS.md`](AGENTS.md); in short: one package does one job, files stay small, comments say why and never what, errors are wrapped with
context and never swallowed, standard library first in Go, `uv` and `ruff` for Python, table tests for logic, and no speculative abstractions or unused
configuration. Where things are: README, "Repository map"; how they fit: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Reporting a bug

The bug form asks for what you did, the AGI and Windows versions (Settings > About, "Copy diagnostics"), your processor and graphics card (file search
is very different with and without one), and the log: `%LOCALAPPDATA%\AGI\logs\agi.log`. The log can contain file names and what you searched for, so
look through it before you paste.

For a security problem, use "Report a vulnerability" on the repository's Security tab instead of a public issue ([`SECURITY.md`](SECURITY.md)).

## What to expect

Replies can take a few days. A quiet week means busy, not ignored. Not every pull request will be merged, even good ones: the project has a direction,
and some changes will not fit it.
