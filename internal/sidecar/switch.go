package sidecar

import (
	"context"
	"sync"
)

// Switch runs a supervisor on demand, so file search can be turned on and off while the launcher
// runs. A supervisor that gave up (StateFailed) is started afresh by the next On.
type Switch struct {
	sup    *Supervisor
	parent context.Context

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{} // closed when the current Run has returned
}

// NewSwitch returns a switch for sup. Nothing runs until On; the engine stops with parent.
func NewSwitch(parent context.Context, sup *Supervisor) *Switch {
	return &Switch{sup: sup, parent: parent}
}

// On starts the engine unless it is already being supervised.
func (w *Switch) On() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.done != nil {
		select {
		case <-w.done: // the last run is over (it failed for good): run again
		default:
			return
		}
	}
	ctx, cancel := context.WithCancel(w.parent)
	done := make(chan struct{})
	w.cancel, w.done = cancel, done
	go func() {
		w.sup.Run(ctx)
		close(done)
	}()
}

// Off stops the engine and waits for its process to end, then reports StateOff.
func (w *Switch) Off() {
	w.mu.Lock()
	cancel, done := w.cancel, w.done
	w.cancel, w.done = nil, nil
	w.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
	w.sup.Off()
}

// Wait returns once the current run has ended (after parent was cancelled, at shutdown).
func (w *Switch) Wait() {
	w.mu.Lock()
	done := w.done
	w.mu.Unlock()
	if done != nil {
		<-done
	}
}
