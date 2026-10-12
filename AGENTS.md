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
- **Small files.** If a file needs scrolling to understand, split it by responsibility.
- **Names over comments.** Comment the *why*, never the *what*. Every Go package has a package doc comment.
- **Errors:** wrap with context (`fmt.Errorf("load config: %w", err)`); never swallow.
- **Go:** stdlib first; `gofmt` + `go vet` clean. **Python:** `uv` for env/deps; `ruff` clean. **Frontend:** Svelte + TypeScript.
- **Tests:** Go `testing` package, table tests for logic. Trivial glue needs no test.

## Constraints

- Windows 11 only (for now). RTX 4050, **6 GB VRAM**: models must fit together. Only models use the GPU; index storage
  and search run on the CPU.
- The hotkey → panel path must never wait on a model. Launcher opens in < 100 ms.
- Privacy: nothing leaves the machine except explicit calls to a configured LLM provider. No telemetry.
- Config in `%APPDATA%\AGI\config.toml`; API keys in Windows Credential Manager, never in files or git.

## Docs

Start here: `docs/README.md` (index), then `docs/STATUS.md` (what is built, what is next) and
`docs/ARCHITECTURE.md` (how it fits together).

- `docs/superpowers/specs/`: design specs (one per feature/phase); the current one is the memory engine spec
- `docs/superpowers/plans/`: implementation plans
- `docs/decisions/`: short decision records (what we chose, alternatives, why)
- Update docs in the same commit as the code they describe.

## Commits

Conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`, `chore:`). Small and focused.
