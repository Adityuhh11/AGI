# Contributing to Omni

Thank you for wanting to help. Omni is a small project with one maintainer and a plan for where it is going, so a few ground rules keep
your time well spent. They are short.

- [Ways to help](#ways-to-help)
- [Open an issue first](#open-an-issue-first)
- [Set up](#set-up)
- [Make the change](#make-the-change)
- [Sign your commits](#sign-your-commits)
- [Using AI tools](#using-ai-tools)
- [Send the pull request](#send-the-pull-request)
- [Report a bug](#report-a-bug)
- [Security](#security)
- [Behaviour](#behaviour)
- [What to expect](#what-to-expect)

## Ways to help

You do not have to write code to make Omni better.

- **Try it on a machine the maintainer does not have**: a graphics card from another vendor, no graphics card at all, a display at 125% or 150%
  scale, a Mac with a different macOS version. "It works on X" and "it breaks on Y" are both useful.
- **Report what breaks**, with the details below. A bug report with a log is worth more than a guess at the fix.
- **Fix the words**: a README line that is wrong, a doc that does not match what the program does.
- **Fix a bug or build a feature**, after the step below.

## Open an issue first

For a fix as much as for a feature. An issue gets you a clear "yes", "not now" or "here is how it should fit" before you spend an evening on it.
Pull requests are by invitation: a pull request that arrives without an issue that says go may be closed with a link back here. That is about the
process, not about the quality of your work.

The exception is the small and obvious: a typo, a broken link, a one-line fix a reviewer can check at a glance. Send those straight away.

For a larger idea, say in the issue what problem you have, what you would change, and what you considered and left out. A few paragraphs are
enough. If it settles a real design question, the answer is written down afterwards as a short decision record in [`docs/decisions/`](docs/decisions/).

## Set up

You need [Go](https://go.dev/dl/) 1.27 or newer, [Node.js](https://nodejs.org/) 24 and npm, and [uv](https://docs.astral.sh/uv/) for the Python engine.

| System | Also needed | Build and run |
|---|---|---|
| Windows 11 | PowerShell, the [Wails v3 CLI](https://v3alpha.wails.io/) (`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.29`) | `.\scripts\build.ps1`, then `.\bin\omni.exe` |
| macOS 13+, Apple Silicon | Xcode command line tools, the Wails CLI as above | `./scripts/build-mac.sh`, then `./bin/omni` |
| Linux | | Not a target. The engine tests and the page tests run there; the launcher's Go code builds on Windows and macOS only |

Checks to run before you push. CI runs the same on every pull request, on a Windows and a Mac machine.

```sh
gofmt -l .                       # prints nothing when clean
go vet ./... && go test ./...    # the launcher; add -race when you touch concurrent code

cd frontend && npm ci && npm test && npm run check            # the page

cd sidecar && uv sync && uv run ruff check && uv run pyright && uv run pytest   # the engine
```

Tests that need the real model or a large index are skipped by themselves (`pytest -m model` runs them, with `OMNI_LLAMA_SERVER` and `OMNI_MODEL_DIR`
set). You do not need a model to work on most of the code.

## Make the change

- **Branch from `dev`** and open the pull request into `dev`. `main` only changes by a pull request from `dev` that the maintainer merges.
- **One thing per pull request**, in small focused commits. Use [conventional commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`,
  `refactor:`, `test:`, `chore:`.
- **Write the test first when you fix a bug**, and watch it fail. Go tests are table tests in the standard `testing` package; Python tests are `pytest`.
- **Keep the code like the code around it.** The rules are in [`AGENTS.md`](AGENTS.md); in short: one package does one job, files stay small,
  comments say *why* and never *what*, errors are wrapped with context and never swallowed, standard library first in Go, and no speculative
  abstractions or unused configuration. `gofmt`, `go vet`, `ruff` and `pyright` must be clean.
- **Keep the two systems apart.** Shared code lives in `x.go`, Windows code only in `x_windows.go`, Mac code only in `x_darwin.go`, tests likewise,
  and no system call in a shared file. The table is in [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md); keep it current in the same commit.
- **A new dependency needs a reason.** Say in the issue what it does that the standard library or what is already here does not, and what it costs
  in size. Every dependency also goes into [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
- **Update the docs that describe the code in the same commit.** [`docs/README.md`](docs/README.md) says which ones.
- **Look at any new window at 125% display scale** on Windows. The program must stay sharp at every scale.
- **If it changes speed, memory or search quality, measure it**, before and after, on the same computer, and put both numbers in the pull request.
  `cmd/panelbench` measures the hotkey path. The search-quality eval (`uv run python -m omni_memory.eval`) runs against the maintainer's private sample
  folder, which you do not have: run it on a folder of your own (`--data`) or describe what you changed, and the maintainer runs the full eval before merging.
- **Nothing that sends data elsewhere fits.** No telemetry, no cloud services, no account. File search runs on the person's computer, and the only
  network calls are the ones in the README's [privacy table](README.md#privacy-and-the-network).

## Sign your commits

Omni uses the [Developer Certificate of Origin](https://developercertificate.org/) (DCO), the same lightweight statement the Linux kernel uses, instead of a
contributor license agreement. Add a `Signed-off-by` line to each commit with `git commit -s`:

```text
Signed-off-by: Your Name <you@example.com>
```

It says that you wrote the change or have the right to submit it, under Omni's [MIT License](LICENSE). The full text:

```text
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right to submit that work with
    modifications, whether created in whole or in part by me, under
    the same open source license (unless I am permitted to submit
    under a different license), as indicated in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

Forgot? `git commit --amend -s` signs the last commit, and `git rebase --signoff origin/dev` signs a whole branch.

## Using AI tools

Using an AI assistant to write code is fine. The rules are the same as for anything else you submit:

- **You are the author.** You have read every line, you understand it, you ran the checks, and you can answer a reviewer's questions about it.
  "The assistant wrote it" is not an answer to a review comment.
- **You sign it** (DCO above), which means you vouch that you have the right to contribute it.
- **Say so in the pull request** when an assistant wrote a significant part. A line is enough. This is not a penalty; it tells the reviewer where to
  look harder.
- **No unrequested sweeps.** A tool that rewrites many files, adds dependencies or "improves" code nobody asked about makes a pull request nobody can
  review. Keep it to the issue.

If you use a coding agent, it reads [`AGENTS.md`](AGENTS.md) automatically; that file is written for it.

## Send the pull request

1. Push your branch and open the pull request into `dev`. The template asks for the issue, what changed and why, and what you checked.
2. CI builds and tests on Windows and on a Mac, and must pass. If it fails, the log says which step; fix it with a new commit.
3. A review can ask for changes. That is normal and not a rejection.
4. The maintainer merges it, or says why not. Not every pull request will be merged, even good ones: the project has a direction, and some
   changes will not fit it.

## Report a bug

Use the bug form on the Issues tab. It asks for what you did, the Omni and system versions (Settings > About > "Copy diagnostics"), your processor
and graphics card (file search is very different with and without one), and the log:

- Windows: `%LOCALAPPDATA%\Omni\logs\omni.log`
- macOS: `~/Library/Logs/Omni/omni.log`

The log can contain file names and what you searched for, so look through it before you paste.

## Security

For a security problem, use **Report a vulnerability** on the repository's Security tab instead of a public issue. See [`SECURITY.md`](SECURITY.md).

## Behaviour

The [Code of Conduct](CODE_OF_CONDUCT.md) in short: be kind and be specific. Disagree with the code, not the person. Harassment, insults and personal attacks have no place in issues, pull requests
or reviews, and the maintainer may edit, close or delete such comments and block the people who write them.

## What to expect

Replies can take a few days. A quiet week means busy, not ignored. If you are unsure whether something is welcome, ask in an issue: it costs nothing.
