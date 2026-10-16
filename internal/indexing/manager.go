// Package indexing keeps the memory engine's index in step with the configured folders: it asks
// the engine to sync every root, follows each job and reports progress. A repeat sync is
// incremental, so unchanged files cost almost nothing and deleted files disappear.
package indexing

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agi/internal/config"
	"agi/internal/sidecar"
)

// Engine is the part of the engine's API the manager uses; *sidecar.Client satisfies it.
type Engine interface {
	Sources(ctx context.Context) ([]sidecar.Source, error)
	AddSource(ctx context.Context, root, id string) (string, error)
	Job(ctx context.Context, id string) (sidecar.Job, error)
	DeleteSource(ctx context.Context, id string) (int, error)
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
	// Files and Chunks are what the configured roots hold once UpToDate.
	Files, Chunks int
	// Message is why a sync failed.
	Message string
}

// String is the one-line form for the tray.
func (s Status) String() string {
	switch s.State {
	case Waiting:
		return "Waiting for the search engine"
	case Indexing:
		name := filepath.Base(s.Root)
		if s.FilesTotal == 0 {
			return "Scanning " + name
		}
		return fmt.Sprintf("Indexing %s: %d / %d files", name, s.FilesDone, s.FilesTotal)
	case UpToDate:
		return fmt.Sprintf("Up to date: %d chunks in %d files", s.Chunks, s.Files)
	default:
		return "Indexing failed: " + s.Message
	}
}

// Manager syncs the roots when the engine is ready and again whenever Rescan is called.
type Manager struct {
	engine   Engine
	roots    []string
	ready    func() bool
	onChange func(Status)

	poll  time.Duration // between job progress checks and readiness checks
	retry time.Duration // after a failed sync

	// deleted says whether a root was removed from a disk that is still there; tests replace it.
	deleted func(root string) bool

	rescan chan struct{}

	mu     sync.Mutex
	status Status
}

// New returns a manager for roots. ready says whether the engine can take work; onChange (may be
// nil) is called, in the manager's goroutine, whenever the status changes.
func New(engine Engine, roots []string, ready func() bool, onChange func(Status)) *Manager {
	return &Manager{
		engine: engine, roots: roots, ready: ready, onChange: onChange,
		poll: 500 * time.Millisecond, retry: time.Minute, deleted: wasDeleted,
		rescan: make(chan struct{}, 1),
		status: Status{State: Waiting},
	}
}

// Status returns the current status.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// Rescan asks for another sync as soon as the current one is over. Asking twice is the same as once.
func (m *Manager) Rescan() {
	select {
	case m.rescan <- struct{}{}:
	default:
	}
}

// Run syncs, then waits for Rescan (or, after a failure, for the retry pause) until ctx ends.
func (m *Manager) Run(ctx context.Context) {
	for {
		var again <-chan time.Time
		if !m.cycle(ctx) {
			again = time.After(m.retry)
		}
		select {
		case <-ctx.Done():
			return
		case <-m.rescan:
		case <-again:
		}
	}
}

func (m *Manager) set(s Status) {
	m.mu.Lock()
	changed := m.status != s
	m.status = s
	m.mu.Unlock()
	if changed && m.onChange != nil {
		m.onChange(s)
	}
}

// cycle runs one full sync and reports whether it succeeded.
func (m *Manager) cycle(ctx context.Context) bool {
	m.set(Status{State: Waiting})
	for !m.ready() {
		select {
		case <-ctx.Done():
			return true
		case <-time.After(m.poll):
		}
	}
	problems, err := m.syncAll(ctx)
	if ctx.Err() != nil {
		return true
	}
	if err != nil {
		problems = append(problems, err.Error())
	}
	if len(problems) > 0 {
		m.set(Status{State: Failed, Message: strings.Join(problems, "; ")})
		log.Print("indexing: ", problems)
		return false
	}
	return true
}

// wasDeleted reports whether root is gone while the folder that held it is still there: the user
// deleted it. A root whose parent is missing too (an unplugged drive) is not a deletion, so its
// index is kept for when the drive is back.
func wasDeleted(root string) bool {
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		return false
	}
	_, err := os.Stat(filepath.Dir(root))
	return err == nil
}

// syncAll forgets sources that are no longer configured (or whose folder was deleted) and syncs
// every other root. A root that fails does not stop the others; its problem is returned.
func (m *Manager) syncAll(ctx context.Context) (problems []string, err error) {
	var roots []string
	ids := make(map[string]bool, len(m.roots))
	for _, root := range m.roots {
		if m.deleted(root) {
			log.Printf("indexing: %s no longer exists, so its index entries are dropped", root)
			continue
		}
		roots = append(roots, root)
		ids[config.SourceID(root)] = true
	}
	sources, err := m.engine.Sources(ctx)
	if err != nil {
		return nil, fmt.Errorf("list indexed folders: %w", err)
	}
	for _, source := range sources {
		if ids[source.ID] {
			continue
		}
		if _, err := m.engine.DeleteSource(ctx, source.ID); err != nil {
			return nil, fmt.Errorf("forget %s: %w", source.Root, err)
		}
	}
	for _, root := range roots {
		if err := m.syncRoot(ctx, root); err != nil {
			problems = append(problems, filepath.Base(root)+": "+err.Error())
		}
	}
	if len(problems) > 0 {
		return problems, nil
	}
	// Totals are read after the last job, so they count what the index holds now.
	if sources, err = m.engine.Sources(ctx); err != nil {
		return nil, fmt.Errorf("list indexed folders: %w", err)
	}
	total := Status{State: UpToDate}
	for _, source := range sources {
		if ids[source.ID] {
			total.Files += source.Files
			total.Chunks += source.Chunks
		}
	}
	m.set(total)
	return nil, nil
}

func (m *Manager) syncRoot(ctx context.Context, root string) error {
	id := config.SourceID(root)
	for {
		jobID, err := m.engine.AddSource(ctx, root, id)
		var busy *sidecar.BusyError
		if err == nil {
			return m.follow(ctx, root, jobID)
		}
		if !errors.As(err, &busy) || busy.JobID == "" {
			return err
		}
		// Another job is running (only after a launcher restart that the engine outlived): let it
		// finish, then submit again.
		if err := m.follow(ctx, root, busy.JobID); err != nil {
			return err
		}
	}
}

// follow reports the job's progress until it ends.
func (m *Manager) follow(ctx context.Context, root, jobID string) error {
	for {
		job, err := m.engine.Job(ctx, jobID)
		if err != nil {
			return err
		}
		switch job.State {
		case sidecar.JobDone:
			if job.Report != nil && len(job.Report.Errors) > 0 {
				log.Printf("indexing %s: %d files could not be read, e.g. %s: %s", root, len(job.Report.Errors),
					job.Report.Errors[0].URI, job.Report.Errors[0].Message)
			}
			return nil
		case sidecar.JobFailed:
			if job.Error != nil {
				return errors.New(*job.Error)
			}
			return errors.New("the job failed")
		}
		m.set(Status{State: Indexing, Root: root, FilesDone: job.FilesDone, FilesTotal: job.FilesTotal})
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.poll):
		}
	}
}
