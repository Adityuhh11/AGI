<!-- CONTRIBUTING.md asks for an issue first, so the change has a "yes" before the work; link it here. -->

Issue: #

What changed and why:

Checked (CI runs the first three on every pull request):

- [ ] `go vet ./...` and `go test ./...`
- [ ] `npm test` and `npm run check` in `frontend/`
- [ ] `uv run ruff check`, `uv run pyright` and `uv run pytest` in `sidecar/`
- [ ] The docs that describe this are updated in the same commit

If search results or speed change: the numbers before and after, on the same computer (`uv run python -m agi_memory.eval`, `cmd/panelbench`).
