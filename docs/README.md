# Documentation index

Start with `STATUS.md`, then `ARCHITECTURE.md`.

| Doc | Answers |
|---|---|
| `STATUS.md` | What is built, what is next, known gaps. Read first. |
| `ARCHITECTURE.md` | How the system fits together: modules, data flow, on-disk formats, invariants. |
| `api.md` | The HTTP contract the Go launcher codes against: process contract, routes, status codes. |
| `RELEASING.md` | How versions are numbered and how a release is cut. |
| `decisions/` | Decision records: what we chose, the alternatives, the numbers behind it. |
| `superpowers/specs/` | Design specs: what we are building and why. |
| `superpowers/plans/` | Implementation plans: how, task by task. |
| `/PRODUCT.md`, `/DESIGN.md` | Product context and the visual system of the launcher panel (read before changing `frontend/`). |
| `eval/` | Retrieval-quality reports, `baseline.json` (the accepted metrics) and `perf.md` (search latency at 100k chunks). |

Rules for keeping docs honest live in `../AGENTS.md`: update the doc in the same commit as the code.
