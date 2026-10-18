# Installing the engine from nothing (spike S2, 2026-10-18)

By hand, in an empty folder under the scratchpad, with `UV_CACHE_DIR`, `UV_PYTHON_INSTALL_DIR` and `UV_PROJECT_ENVIRONMENT` pointing into it
(nothing in the repo or the user profile). The engine source was `sidecar/` with `torch`, `transformers` and `sentence-transformers` removed
from `pyproject.toml` (and the PyTorch index entry), a fresh `uv lock` (34 packages), then the steps the installer will run.

## Findings

| Step | Result |
|---|---|
| uv 0.13.1 zip from GitHub | 15.3 MB download, `uv.exe` + `uvx.exe` + `uvw.exe` 32 MB unpacked; 5 s |
| `uv lock` without torch | 34 packages, under a second to resolve |
| `uv sync --frozen --no-dev --python 3.12` with `UV_PYTHON_PREFERENCE=only-managed` | **15 s**; uv downloaded its own CPython 3.12.15 (about 23 MB) and the wheels (cache 79 MB in all); python 69 MB + venv 84 MB = **153 MB on disk**; `import agi_memory, fastapi, numpy, PIL, pypdfium2, uvicorn` works |
| Total engine download besides llama.cpp and the model | about 15 + 80 = **95 MB** |

Two things that would have broken a clean machine, now rules for the installer:

- **Pin the Python.** Without `--python 3.12`, `only-managed` picked CPython 3.15.0 and the sync did not produce a usable environment (the import failed). Without
  `only-managed` uv used the Python 3.13 already on this laptop, which a friend may not have. The installer passes `--python 3.12` and sets `UV_PYTHON_PREFERENCE=only-managed`.
- **`--frozen`** needs the lock file next to `pyproject.toml` (the exe embeds both); the lock must be regenerated in block D when torch leaves the project.

## Pins for `internal/enginesetup` (block E)

| Artifact | Source | Size | SHA-256 |
|---|---|---|---|
| uv 0.13.1 | `https://github.com/astral-sh/uv/releases/download/0.13.1/uv-x86_64-pc-windows-msvc.zip` | 15,308,685 | `3b2661a463a5baea7f5076ea2b25ab71d77f36a6c0d809e5350651792f80a672` |
| llama.cpp b11719, Vulkan | `https://github.com/ggml-org/llama.cpp/releases/download/b11719/llama-b11719-bin-win-vulkan-x64.zip` | 32,306,255 | `08dde7f4133443ac629431c35f0435e267ca73251fc002cb95e10625424e2963` |
| text model | `https://huggingface.co/ggml-org/embeddinggemma-2-GGUF/resolve/bfcd298762cc34d0357ece5ebdd31791a3a374d8/embeddinggemma-2-Q8_0.gguf` | 309,855,456 | `2188ac1deca4b77dffefd603c2776a9d76d9d74ec01841392982ebb840b09135` |
| vision projector | `https://huggingface.co/ggml-org/embeddinggemma-2-GGUF/resolve/bfcd298762cc34d0357ece5ebdd31791a3a374d8/mmproj-embeddinggemma-2-Q8_0.gguf` | 554,821,024 | `c4a8a52691ecef40618438928bdf9e68379b854e24166f292592353db0aab64f` |

The uv, llama.cpp and model digests are the ones the hosts publish (GitHub's asset `digest`, Hugging Face's LFS `oid`); the llama.cpp zip and both GGUF
files that were downloaded for the runtime spike hash to the same values locally. Total download for file search: 15.3 + about 80 + 32.3 + 864.7 = **about 1.0 GB**.

Not measured: Hugging Face `Range` requests (block E's tests use a local server; the real host is checked in E3), and an install behind a proxy.
