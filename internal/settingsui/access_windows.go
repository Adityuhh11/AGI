package settingsui

// Windows has no permission to ask for before reading a folder.

func blocked(string) bool { return false }

// blockedText is never shown on Windows.
const blockedText = ""

func openAccessSettings() error { return nil }
