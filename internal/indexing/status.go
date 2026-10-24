package indexing

import (
	"fmt"
	"path/filepath"
	"time"
)

// FolderStatus is one configured folder as the settings window lists it.
type FolderStatus struct {
	Root string
	// State is Waiting (not started), Indexing (FilesDone of FilesTotal), UpToDate (Files and Chunks) or Failed.
	State                 State
	FilesDone, FilesTotal int
	Files, Chunks         int
	Message               string
	// Remaining is the estimate of what is left of this folder's job, 0 while there is none.
	Remaining time.Duration
}

// State is where the manager is.
type State string

const (
	// Waiting: the engine is not ready yet (its model is loading).
	Waiting State = "waiting"
	// Indexing: a root is being synced.
	Indexing State = "indexing"
	// UpToDate: every root was synced; the index matches the disk as of the last sync.
	UpToDate State = "uptodate"
	// Failed: a sync did not finish; Message says why. It is tried again after a pause.
	Failed State = "failed"
)

// Status is what the tray and the UI show.
type Status struct {
	State State
	// Root, FilesDone and FilesTotal describe the job in progress (FilesTotal is 0 while scanning).
	Root       string
	FilesDone  int
	FilesTotal int
	// Remaining is the estimate of what is left of that job, rounded to the minute; 0 while there is none.
	Remaining time.Duration
	// Files and Chunks are what the configured roots hold once UpToDate.
	Files, Chunks int
	// Message is why a sync failed.
	Message string
	// Paused says that indexing is stopped on request; a job in progress waits at its next batch.
	Paused bool
}

// String is the one-line form for the tray.
func (s Status) String() string {
	if s.Paused && s.State == Indexing {
		return "Indexing paused"
	}
	switch s.State {
	case Waiting:
		return "Waiting for the search engine"
	case Indexing:
		name := filepath.Base(s.Root)
		if s.FilesTotal == 0 {
			return "Scanning " + name
		}
		return fmt.Sprintf("Indexing %s: %d / %d files%s", name, s.FilesDone, s.FilesTotal, LeftText(s.Remaining))
	case UpToDate:
		return fmt.Sprintf("Up to date: %d chunks in %d files", s.Chunks, s.Files)
	default:
		return "Indexing failed: " + s.Message
	}
}
