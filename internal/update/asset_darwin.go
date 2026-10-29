package update

// assetName is empty on a Mac: there is no one-click update until the app is signed and notarized (docs/superpowers/specs/2026-10-29-macos-design.md,
// section 0). The check still finds a newer release, and the page offers its address instead of an install button; a Mac user deletes the
// app and downloads the new one.
func assetName(string) string { return "" }
