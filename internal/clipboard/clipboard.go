// Package clipboard remembers what the user copied, searches it, and pastes it back into the program the
// search bar was opened from. Watching the clipboard and typing into other programs are Windows calls (the
// _windows.go files); the history, its limits and the order of a paste are plain Go and are tested without a screen.
package clipboard
