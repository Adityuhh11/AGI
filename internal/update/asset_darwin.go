package update

// assetName is empty on a Mac: there is no one-click update until the app is signed and notarized (docs/decisions/0010-macos-distribution-without-apple-account.md). The check still finds a newer release, and the page offers its address instead of an install button; a Mac user deletes the
// app and downloads the new one.
func assetName(string) string { return "" }
