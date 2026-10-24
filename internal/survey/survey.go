// Package survey counts what the configured folders hold, in the background, so Settings and the setup guide can say
// how big each folder is and how long indexing it will take before anything is indexed. The count is the engine's
// own scan (it knows what it would skip, .gitignore included) without opening a file, so it is fast and never fetches
// a cloud-only file.
package survey

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"agi/internal/config"
	"agi/internal/sidecar"
)

// Engine is the part of the engine's API the manager uses; *sidecar.Client satisfies it.
type Engine interface {
	Survey(ctx context.Context, root string, exclude []string) (sidecar.Survey, error)
}

// State is where one folder's survey is.
type State string

const (
	// Pending: not asked yet (the engine is not up, or other folders are first).
	Pending State = "pending"
	// Running: the engine is counting it.
	Running State = "running"
	// Done: Survey holds the count.
	Done State = "done"
	// Failed: the engine could not count it; Message says why, and it is tried again.
	Failed State = "failed"
	// Unavailable: the engine installed is older than the launcher and cannot count. Updating it fixes that.
	Unavailable State = "unavailable"
)

// Result is one folder's survey.
type Result struct {
	State   State
	Survey  sidecar.Survey
	Message string
}

type entry struct {
	exclude string // the folders left out that the count was made with
	Result
}

// Manager surveys every configured folder once the engine is up, and again for a folder whose left-out folders change.
type Manager struct {
	engine Engine
	ready  func() bool

	poll  time.Duration // between checks for an engine that is not up yet
	retry time.Duration // before asking again after a failure

	wake chan struct{}

	mu      sync.Mutex
	roots   []string
	exclude []string
	results map[string]entry
}

// New returns a manager; ready says whether the engine can answer.
func New(engine Engine, ready func() bool) *Manager {
	return &Manager{engine: engine, ready: ready, poll: time.Second, retry: 30 * time.Second, wake: make(chan struct{}, 1), results: map[string]entry{}}
}

// Set replaces the folders to count and the folders inside them to leave out. A folder already counted with the same
// left-out folders keeps its result.
func (m *Manager) Set(roots, exclude []string) {
	m.mu.Lock()
	m.roots, m.exclude = slices.Clone(roots), slices.Clone(exclude)
	for root := range m.results {
		if !slices.Contains(m.roots, root) {
			delete(m.results, root)
		}
	}
	m.mu.Unlock()
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// Results is where each configured folder's survey is.
func (m *Manager) Results() map[string]Result {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]Result, len(m.roots))
	for _, root := range m.roots {
		if e, ok := m.results[root]; ok && e.exclude == m.leftOut(root) {
			out[root] = e.Result
		} else {
			out[root] = Result{State: Pending}
		}
	}
	return out
}

// leftOut is the key of the folders inside root that are left out; the caller holds the lock.
func (m *Manager) leftOut(root string) string {
	return strings.Join(config.ExcludedUnder(root, m.exclude), "\n")
}

// next is a folder that has no current result, with what it is to be counted with.
func (m *Manager) next(now time.Time, failedAt map[string]time.Time) (root string, exclude []string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.roots {
		e, found := m.results[r]
		current := found && e.exclude == m.leftOut(r)
		if current && (e.State == Done || e.State == Unavailable) {
			continue
		}
		if current && e.State == Failed && now.Sub(failedAt[r]) < m.retry {
			continue
		}
		return r, config.ExcludedUnder(r, m.exclude), true
	}
	return "", nil, false
}

func (m *Manager) store(root string, exclude []string, r Result) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if slices.Contains(m.roots, root) {
		m.results[root] = entry{exclude: strings.Join(exclude, "\n"), Result: r}
	}
}

// Run counts the folders, one at a time, until ctx ends.
func (m *Manager) Run(ctx context.Context) {
	failedAt := map[string]time.Time{}
	wasReady := false
	for {
		now := time.Now()
		ready := m.ready()
		if ready && !wasReady {
			m.forgetUnavailable() // the engine may have been updated since it said it could not count
		}
		wasReady = ready
		if root, exclude, ok := m.next(now, failedAt); ok && ready {
			m.store(root, exclude, Result{State: Running})
			survey, err := m.engine.Survey(ctx, root, exclude)
			var api *sidecar.APIError
			switch {
			case ctx.Err() != nil:
				return
			case errors.As(err, &api) && api.Status == http.StatusNotFound:
				m.store(root, exclude, Result{State: Unavailable})
			case err != nil:
				failedAt[root] = time.Now()
				m.store(root, exclude, Result{State: Failed, Message: err.Error()})
			default:
				m.store(root, exclude, Result{State: Done, Survey: survey})
			}
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-m.wake:
		case <-time.After(m.poll):
		}
	}
}

func (m *Manager) forgetUnavailable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for root, e := range m.results {
		if e.State == Unavailable {
			delete(m.results, root)
		}
	}
}
