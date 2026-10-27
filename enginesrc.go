package omni

import "embed"

// EngineSource is the memory engine's Python source (the repository's sidecar folder: pyproject.toml,
// uv.lock and src/), embedded so the launcher can install exactly the engine it was built with. `all:`
// is needed because Go skips files that start with an underscore, such as __init__.py; scripts/build.ps1
// deletes __pycache__ folders first so no compiled files are embedded.
//
//go:embed sidecar/pyproject.toml sidecar/uv.lock all:sidecar/src
var EngineSource embed.FS
