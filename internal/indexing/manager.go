// Package indexing keeps the memory engine's index in step with the configured folders: it asks
// the engine to sync every root, follows each job and reports progress. A repeat sync is
// incremental, so unchanged files cost almost nothing and deleted files disappear.
package indexing

import (
	"context"
	"slices"
	"sync"
	"time"

	"agi/internal/sidecar"
)

// Engine is the part of the engine's API the manager uses; *sidecar.Client satisfies it.
type Engine interface {
	Sources(ctx context.Context) ([]sidecar.Source, error)
	AddSource(ctx context.Context, root, id string, exclude []string) (string, error)
	Job(ctx context.Context, id string) (sidecar.Job, error)
	DeleteSource(ctx context.Context, id string) (int, error)
	SetDuty(ctx context.Context, duty float64) error
	Pause(ctx context.Context) error
	Resume(ctx context.Context) error
}

// Manager syncs the roots when the engine is ready and again whenever Rescan is called.
type Manager struct {
	engine   Engine
	ready    func() bool
	onChange func(Status)

	poll  time.Duration // between job progress checks and readiness checks
	retry time.Duration // the pause after a first failed sync; it doubles with each failure in a row

	// deleted says whether a root was removed from a disk that is still there; tests replace it.
	deleted func(root string) bool

	rescan chan struct{}
	wake   chan struct{} // a folder changed on disk (see Changed)

	mu      sync.Mutex
	status  Status
	roots   []string        // the folders to index
	exclude []string        // folders inside the roots to leave out
	dirty   map[string]bool // roots whose folders changed since they were last synced
	pace    string          // fast, balanced or gentle
	paused  bool
	states  map[string]FolderStatus // where each folder is, for the settings window
}

// New returns a manager for roots. ready says whether the engine can take work; onChange (may be
// nil) is called, in the manager's goroutine, whenever the status changes.
func New(engine Engine, roots []string, ready func() bool, onChange func(Status)) *Manager {
	return &Manager{
		engine: engine, roots: slices.Clone(roots), ready: ready, onChange: onChange,
		poll: 500 * time.Millisecond, retry: time.Minute, deleted: wasDeleted,
		rescan: make(chan struct{}, 1),
		wake:   make(chan struct{}, 1),
		dirty:  map[string]bool{},
		status: Status{State: Waiting},
		pace:   "balanced", states: map[string]FolderStatus{},
	}
}

// Folders lists the configured folders with where each one is.
func (m *Manager) Folders() []FolderStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]FolderStatus, 0, len(m.roots))
	for _, root := range m.roots {
		f, ok := m.states[root]
		if !ok {
			f = FolderStatus{Root: root, State: Waiting}
		}
		out = append(out, f)
	}
	return out
}

func (m *Manager) failed(root string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.states[root].State == Failed
}

func (m *Manager) setFolder(f FolderStatus) {
	m.mu.Lock()
	m.states[f.Root] = f
	m.mu.Unlock()
}

// Status returns the current status.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.status
	s.Paused = m.paused
	return s
}

// SetExclude sets the folders to leave out of the roots before Run starts; SetFolders is for a
// manager that is running.
func (m *Manager) SetExclude(exclude []string) {
	m.mu.Lock()
	m.exclude = slices.Clone(exclude)
	m.mu.Unlock()
}

// SetFolders replaces the folders to index and the folders to leave out of them, and asks for a sync.
// A folder that is no longer listed is dropped from the index by that sync.
func (m *Manager) SetFolders(roots, exclude []string) {
	m.mu.Lock()
	m.roots, m.exclude = slices.Clone(roots), slices.Clone(exclude)
	m.mu.Unlock()
	m.Rescan()
}

// folders returns the current folders to index and to leave out.
func (m *Manager) folders() (roots, exclude []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.roots), slices.Clone(m.exclude)
}

// Rescan asks for another sync as soon as the current one is over. Asking twice is the same as once.
func (m *Manager) Rescan() {
	select {
	case m.rescan <- struct{}{}:
	default:
	}
}

// Changed says that something changed under root (one of the configured folders): it is synced soon,
// on its own, without touching the other roots.
func (m *Manager) Changed(root string) {
	m.mu.Lock()
	m.dirty[root] = true
	m.mu.Unlock()
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// takeDirty returns the roots that changed since the last call, and forgets them.
func (m *Manager) takeDirty() map[string]bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	taken := m.dirty
	m.dirty = map[string]bool{}
	return taken
}

// maxRetryPause is the longest the manager waits before trying a failed sync again.
const maxRetryPause = 15 * time.Minute

// retryPause is how long to wait after failures syncs in a row have failed: the first pause, doubled
// each time, up to maxRetryPause. A sync that fails for a lasting reason is not worth hammering.
func (m *Manager) retryPause(failures int) time.Duration {
	pause := m.retry
	for i := 1; i < failures && pause < maxRetryPause; i++ {
		pause *= 2
	}
	return min(pause, maxRetryPause)
}

// Run syncs everything, then syncs again when asked: all roots on Rescan (and, after a failure,
// when the retry pause is over), just the changed roots on Changed. It returns when ctx ends.
func (m *Manager) Run(ctx context.Context) {
	failures := 0 // syncs in a row that failed
	run := func(only map[string]bool) {
		if m.cycle(ctx, only) {
			failures = 0
		} else {
			failures++
		}
	}
	run(nil)
	for {
		var retry <-chan time.Time
		if failures > 0 {
			retry = time.After(m.retryPause(failures))
		}
		select {
		case <-ctx.Done():
			return
		case <-m.rescan:
			run(nil)
		case <-retry:
			run(nil)
		case <-m.wake:
			if changed := m.takeDirty(); len(changed) > 0 {
				run(changed)
			}
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
