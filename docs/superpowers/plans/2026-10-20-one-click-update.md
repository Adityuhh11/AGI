# One-click update: plan

Spec: `docs/superpowers/specs/2026-10-20-one-click-update-design.md`. Branch `feat/self-update`, cut from `dev`. One commit per
task, docs in the same commit as the code, `go vet`, `go test ./...`, `svelte-check` and `npm test` green before each. No AI attribution.

**U1 `internal/update`: the asset.** `Release` gains `Asset{Name, URL, Size, SHA256}`: the `omni-<version>-windows-x64.zip` of the
release (a missing zip, or one with no `sha256:` digest, leaves `Asset` empty, which means "release page only"). Tests: the real
answer's shape, no digest, no zip, an unexpected digest algorithm.

**U2 `internal/update/install.go`.** `Install(ctx, asset, exePath, progress)`: download into a temporary folder (at most 200 MB,
`https://github.com/Adityuhh11/Omni/releases/download/` only, cancellable, progress in bytes), verify the SHA-256, extract the entry named exactly
`omni.exe` into the program's folder as `omni.exe.new`, swap with rollback and a few retries. `Installer` keeps the state the page
reads (`idle`, `downloading`, `verifying`, `installing`, `done`, `failed`, with bytes and the error) and refuses a second start.
Tests with a local server and temporary folders: success, wrong digest, truncated download, too big, an address outside the project,
a zip with no `omni.exe` / a nested one / a path in it, an unwritable folder, rollback when the second rename fails, cancel,
double start.

**U3 restart.** `sidecar.KillWithLauncher` adds `JOB_OBJECT_LIMIT_BREAKAWAY_OK`; `update.StartNew(exe, pid)` creates the new launcher with
`CREATE_BREAKAWAY_FROM_JOB | DETACHED_PROCESS` and `--wait-for <pid>`; `update.WaitForExit(pid, timeout)`; `cmd/omni` handles
`--wait-for` before the single-instance check and deletes `omni.exe.old`. A real test (a test program that starts a copy of itself and
exits) shows the child outlives its parent inside the job.

**U4 service and page.** `settingsui`: `StartUpdate`, `CancelUpdate`, `UpdateProgress`; when the install is done the service
calls the restart function `main` gives it. About page: "Update to X" next to the release-page link, a progress line, the error,
the fallback. Tests: the service with a fake installer (refuses without a found release or asset, refuses twice, restarts after
`done`, not after `failed`).

**U5 end to end against the real v0.3.0** (the evidence in `docs/eval/2026-10-20-self-update.md`): a build stamped 0.0.1 in a
scratch folder, a clean profile, the owner's launcher stepped aside by process id and brought back; check, update, restart; the
program that runs afterwards is 0.3.0, `.old` is gone, one launcher runs, the settings and pins are as they were. Also an update
from a read-only folder shows the message and leaves the program as it was.

**U6 docs.** ADR 0005 updated, `AGENTS.md` privacy line, STATUS, the README's line on updating.
