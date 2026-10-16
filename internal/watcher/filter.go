package watcher

import "strings"

// skippedFolders are never indexed by the engine (its deny list), so changes inside them are noise:
// a build or an install can write thousands of files there. Folders that start with a dot (.git,
// .venv, .next, .cache, .vscode) are skipped too.
var skippedFolders = map[string]bool{"node_modules": true, "venv": true, "__pycache__": true}

// partialSuffixes mark files that are still being written or are an editor's scratch copy; the
// finished file arrives under its real name, and that event is the one that counts.
var partialSuffixes = []string{".tmp", ".crdownload", ".part", ".partial", ".swp", ".swx", "~"}

// relevant reports whether a change at rel, a path relative to a watched folder, can alter what the
// index should hold.
func relevant(rel string) bool {
	rel = strings.Trim(strings.ReplaceAll(rel, "/", `\`), `\`)
	if rel == "" {
		return false
	}
	parts := strings.Split(rel, `\`)
	for _, part := range parts {
		if strings.HasPrefix(part, ".") || skippedFolders[strings.ToLower(part)] {
			return false
		}
	}
	name := strings.ToLower(parts[len(parts)-1])
	if strings.HasPrefix(name, "~$") { // Office's lock file next to an open document
		return false
	}
	for _, suffix := range partialSuffixes {
		if strings.HasSuffix(name, suffix) {
			return false
		}
	}
	return true
}
