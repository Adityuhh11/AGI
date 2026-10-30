<!-- CONTRIBUTING.md asks for an issue first, so the change has a "yes" before the work; link it here. -->

Issue: #

What changed and why:

Checked (CI runs the first three on every pull request):

- [ ] `gofmt -l .`, `go vet ./...` and `go test ./...`
- [ ] `npm test` and `npm run check` in `frontend/`
- [ ] `uv run ruff check`, `uv run pyright` and `uv run pytest` in `sidecar/`
- [ ] The docs that describe this are updated in the same commit
- [ ] Every commit is signed off (`git commit -s`, the [DCO](../CONTRIBUTING.md#sign-your-commits))

If search results or speed change: the numbers before and after, on the same computer (`cmd/panelbench` for the hotkey path; the search eval needs the
maintainer's sample folder, so describe the change and the maintainer runs it).

AI tools: none / an assistant wrote a significant part (say which parts, and that you have read and run it).
