# Documentation index

New here? The repository's `README.md` explains what Omni is, how to install it and how it works. Contributors start with `STATUS.md`, then `ARCHITECTURE.md`.

| Doc | Answers |
|---|---|
| `STATUS.md` | What is built, what is next, known gaps. Read first. |
| `ARCHITECTURE.md` | How the system fits together: modules, data flow, on-disk formats, invariants, and which code is for Windows and which for a Mac. |
| `api.md` | The HTTP contract the Go launcher codes against: process contract, routes, status codes. |
| `PERFORMANCE.md` | Measured speed and resource use, with a graphics card and without one; how each number was taken. |
| `DISTRIBUTION.md` | Why Windows warns about an unsigned program, and the route to a signed, trusted install. |
| `RELEASING.md` | How versions are numbered and how a release is cut. |
| `INSTALL-MAC.md` | Installing, updating and removing Omni on a Mac, and the permissions macOS asks for. |
| `decisions/` | Decision records: what we chose, the alternatives, the numbers behind it. |
| `/DESIGN.md` | The visual system of the launcher panel (read before changing `frontend/`). |
| `eval/` | Retrieval-quality reports, `baseline.json` (the accepted metrics) and `perf.md` (search latency at 100k chunks). |

Rules for keeping docs honest live in `../AGENTS.md`: update the doc in the same commit as the code.
