# AGENTS.md

Rules for any coding agent working in this repository (Claude Code imports this file from `CLAUDE.md`). They are the same rules a human
contributor follows; [`CONTRIBUTING.md`](CONTRIBUTING.md) has the process around them (issue first, setup, pull requests).

Omni is a local-first launcher for Windows 11 and macOS 13+ (Apple Silicon): a Go core, Wails v3 with a Svelte + TypeScript page for the
UI, and a Python sidecar (the "memory engine") that runs local models for file search.

## Orient first

`docs/README.md` is the index. Read `docs/STATUS.md` (what is built, what is next) and `docs/ARCHITECTURE.md` (how it fits together,
the invariants, which code is for which system) before changing anything non-trivial. `DESIGN.md` comes first for work in `frontend/`.

## Checks

Run what you touched before you say it is done; CI runs all of it.

```sh
gofmt -l .                       # prints nothing when clean
go vet ./... && go test ./...    # launcher; add -race for concurrent code
cd frontend && npm ci && npm test && npm run check     # page
cd sidecar && uv sync && uv run ruff check && uv run pyright && uv run pytest   # engine
```

Build: `.\scripts\build.ps1` (Windows) or `./scripts/build-mac.sh` (Mac). Tests that need a model or a large index skip themselves.

## How we work

- **Measure, don't guess.** A choice that affects speed, memory or search quality gets a benchmark before and after, on the same machine
  (`cmd/panelbench` for the hotkey path; the retrieval eval in `sidecar`), and the numbers go in the pull request or a record in `docs/eval/`.
- **Design before code for a feature.** Think it through in writing first: the issue is the place (`docs/specs/` is git-ignored, for a private
  scratch spec); a design choice worth keeping gets a record in `docs/decisions/`. Small fixes skip this. Fix a bug test-first.
- **No slop.** Every file, function and dependency earns its place: no speculative abstractions, no "for later" scaffolding, no unused
  configuration. A new dependency needs a reason, and goes into `THIRD_PARTY_NOTICES.md`.
- **Stay in scope.** Do what the issue asks. No drive-by rewrites, renames or "improvements" across files nobody mentioned.

## Code rules

- **Modular:** one package = one job. `internal/<name>/` packages expose a small API; their internals stay private.
- **Two systems, kept apart:** shared logic in `x.go`, Windows code only in `x_windows.go`, Mac code only in `x_darwin.go` (tests likewise);
  no system API in a shared file. The table is in `docs/ARCHITECTURE.md`; keep it current in the same commit.
- **Small files.** If a file needs scrolling to understand, split it by responsibility.
- **Names over comments.** Comment the *why*, never the *what*. Every Go package has a package doc comment.
- **Errors:** wrap with context (`fmt.Errorf("load config: %w", err)`); never swallow.
- **Go:** standard library first; `gofmt` and `go vet` clean. **Python:** `uv` for the environment and dependencies; `ruff` and `pyright` clean.
  **Frontend:** Svelte + TypeScript.
- **Tests:** Go `testing` package, table tests for logic; `pytest` for the engine; the page has its own. Trivial glue needs no test.

## Constraints

- Windows 11 and macOS 13+ on Apple Silicon. Linux is not a target (the engine and page tests run there; the launcher does not build).
- The model budget is 6 GB of graphics memory on a laptop GPU: the models must fit together. Only models use the GPU; index storage and search
  run on the CPU.
- The hotkey → panel path never waits on a model. The launcher opens in under 100 ms.
- Sharp at every display scale: the Windows process is per-monitor DPI aware (declared in `build/windows/omni.manifest`) and
  `scripts/build.ps1` fails a build that is not (`docs/decisions/0006-dpi-awareness.md`). Look at any new window at 125%.
- Privacy: nothing leaves the machine except explicit calls to a configured LLM provider and requests the user starts or switches on (the
  file-search and Omni AI downloads, the opt-in update check and the update the user presses to install, `docs/decisions/0005-update-check.md`).
  No telemetry. The README's privacy table is the full list; do not add to it casually.
- Settings live in `config.toml` in the per-user app folder (`%APPDATA%\Omni` on Windows). API keys go in the system credential store (Windows
  Credential Manager, macOS Keychain), never in files or git.

## Docs

Update the docs that describe the code in the same commit as the code (`docs/README.md` says which). Decision records are short: what we chose,
the alternatives, why. Reports of measurements go in `docs/eval/`. `Sample Data/` is git-ignored and never committed.

## Branches and commits

- `main` holds the stable product and changes only by a pull request that the maintainer merges. Work on a branch cut from `dev`, and open the
  pull request into `dev`. Never push to `main`.
- Conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`), small and focused.
- Commits are signed off with the [DCO](CONTRIBUTING.md#sign-your-commits) by the human who submits them (`git commit -s`). An agent does not
  sign off for them and adds no `Co-Authored-By` or "Generated with" trailers to commits or pull request descriptions; the pull request template
  asks the human to say when an assistant wrote a significant part.
