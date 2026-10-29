package update

// assetName is the file of a release the one-click update installs on Windows: the zip holding omni.exe.
func assetName(version string) string { return "omni-" + version + "-windows-x64.zip" }
