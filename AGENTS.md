# AGENTS.md

Shared project rules for every coding agent (Claude Code imports this file from `CLAUDE.md`).

Personal, local-first AI launcher for Windows. Go core + Wails v3 + Svelte UI + Python sidecar for local models.

## How we work

- **Product first.** Build the best product we can; learning is a side effect, not a constraint. Don't slow down or simplify for teaching. Briefly explain non-obvious decisions; the owner is still new to Go, Svelte, and Wails, so say what's idiomatic there. Custom-built components (e.g. our own vector store) are welcome when they beat off-the-shelf for this use case.
- **Measure, don't guess.** Choices that affect speed, memory, or quality get a benchmark on real data (`Sample Data/`, git-ignored) before they go in a spec.
- **Spec → plan → build.** New features get a spec in `docs/superpowers/specs/` and a plan before code. Small fixes skip this.
- **No slop.** Every file, function, and dependency must earn its place. No speculative abstractions, no "for later" scaffolding, no unused config.

## Code rules

- **Modular:** one package = one job. `internal/<name>/` packages expose a small API; their internals stay private.
- **Two systems, kept apart:** shared logic in `x.go`, Windows code only in `x_windows.go`, Mac code only in `x_darwin.go` (tests likewise); no
  system API in a shared file. The table is in `docs/ARCHITECTURE.md`; keep it current in the same commit.
- **Small files.** If a file needs scrolling to understand, split it by responsibility.
- **Names over comments.** Comment the *why*, never the *what*. Every Go package has a package doc comment.
- **Errors:** wrap with context (`fmt.Errorf("load config: %w", err)`); never swallow.
- **Go:** stdlib first; `gofmt` + `go vet` clean. **Python:** `uv` for env/deps; `ruff` clean. **Frontend:** Svelte + TypeScript.
- **Tests:** Go `testing` package, table tests for logic. Trivial glue needs no test.

## Constraints

- Windows 11 only (for now). RTX 4050, **6 GB VRAM**: models must fit together. Only models use the GPU; index storage
  and search run on the CPU.
- The hotkey → panel path must never wait on a model. Launcher opens in < 100 ms.
- Sharp at every display scale: the process is per-monitor DPI aware (declared in `build/windows/omni.manifest`) and
  `scripts/build.ps1` fails a build that is not (`docs/decisions/0006-dpi-awareness.md`). Look at any new window at 125%.
- Privacy: nothing leaves the machine except explicit calls to a configured LLM provider and requests the user
  starts or switches on (the file-search and Omni AI downloads, the opt-in update check and the update the user presses to install, `docs/decisions/0005-update-check.md`).
  No telemetry.
- Config in `%APPDATA%\Omni\config.toml`; API keys in Windows Credential Manager, never in files or git.

## Docs

Start here: `docs/README.md` (index), then `docs/STATUS.md` (what is built, what is next) and
`docs/ARCHITECTURE.md` (how it fits together).

- `docs/superpowers/specs/`: design specs (one per feature/phase); the current one is the memory engine spec
- `docs/superpowers/plans/`: implementation plans
- `docs/decisions/`: short decision records (what we chose, alternatives, why)
- Update docs in the same commit as the code they describe.

## Branches

`main` holds the stable product and only changes by pull request that the owner merges. Work happens on `dev`, or on a
feature branch cut from `dev` and merged back into it. After `dev` goes into `main`, `dev` is brought up to date with `main`
(rebase or merge) before more work starts. Never push to `main` directly.

## Commits

Conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`, `chore:`). Small and focused.

No AI attribution: never add `Co-Authored-By` or 'Generated with' trailers to commits or PR descriptions.
