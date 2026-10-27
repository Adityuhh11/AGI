// Package omni holds what has to sit at the repository root: the built frontend, embedded so the
// launcher ships as a single executable (go:embed cannot reach outside its own folder).
package omni

import "embed"

// Frontend is the Svelte build (`frontend/dist`), produced by scripts/build.ps1.
//
//go:embed all:frontend/dist
var Frontend embed.FS
