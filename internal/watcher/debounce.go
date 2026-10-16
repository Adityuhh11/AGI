package watcher

import (
	"sync"
	"time"
)

// debouncer turns a burst of events for one key into a single call. A camera import or an unzip
// is thousands of events; the call comes once nothing has happened for quiet, or longest after the
// burst began if it never goes quiet (a build writing files all the time must not starve the sync).
type debouncer struct {
	quiet, longest time.Duration
	fire           func(key string)

	mu      sync.Mutex
	pending map[string]*burst
	stopped bool
}

type burst struct {
	began time.Time
	timer *time.Timer
}

func newDebouncer(quiet, longest time.Duration, fire func(key string)) *debouncer {
	return &debouncer{quiet: quiet, longest: longest, fire: fire, pending: map[string]*burst{}}
}

// poke records an event for key.
func (d *debouncer) poke(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return
	}
	now := time.Now()
	b := d.pending[key]
	if b == nil {
		b = &burst{began: now}
		d.pending[key] = b
	} else {
		b.timer.Stop()
	}
	wait := d.quiet
	if left := b.began.Add(d.longest).Sub(now); left < wait {
		wait = max(left, 0)
	}
	b.timer = time.AfterFunc(wait, func() { d.release(key, b) })
}

func (d *debouncer) release(key string, b *burst) {
	d.mu.Lock()
	if d.stopped || d.pending[key] != b {
		d.mu.Unlock()
		return
	}
	delete(d.pending, key)
	d.mu.Unlock()
	d.fire(key)
}

// stop cancels everything still waiting; no call is made after it returns.
func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopped = true
	for _, b := range d.pending {
		b.timer.Stop()
	}
	d.pending = map[string]*burst{}
}
