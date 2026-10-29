// Package watcher tells the launcher when something changed under a folder it indexes, so a new,
// moved or deleted file reaches the index in seconds instead of at the next manual rescan.
//
// Watching a tree is the system's: ReadDirectoryChangesW on Windows (watch_windows.go), which watches a whole tree through one handle,
// and FSEvents on a Mac (watch_darwin.go). What they share is here: events inside folders the engine never indexes (hidden ones,
// node_modules, ...) and partial files are dropped (filter.go), and a burst of events becomes one call once it has gone quiet (debounce.go).
package watcher
