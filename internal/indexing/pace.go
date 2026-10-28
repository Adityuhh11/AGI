package indexing

import (
	"context"
	"log"
	"time"
)

// Duty is the share of the time indexing may keep the GPU busy at each speed of the settings. Measured in
// docs/eval/2026-10-18-pace.md: Balanced takes about twice as long as Fast, Gentle about six times.
func Duty(pace string) float64 {
	switch pace {
	case "fast":
		return 1.0
	case "gentle":
		return 0.2
	default: // balanced
		return 0.5
	}
}

// SetPace sets how hard indexing works (fast, balanced or gentle) and tells the engine if it is up. An engine
// that is not up yet, or that restarts, is told at the start of the next sync.
func (m *Manager) SetPace(pace string) {
	m.mu.Lock()
	m.pace = pace
	m.mu.Unlock()
	m.tell(func(ctx context.Context) error { return m.engine.SetDuty(ctx, Duty(pace)) })
}

// Pause stops indexing at the next batch; searching goes on.
func (m *Manager) Pause() { m.setPaused(true) }

// Resume lets indexing go on.
func (m *Manager) Resume() { m.setPaused(false) }

func (m *Manager) setPaused(paused bool) {
	m.mu.Lock()
	changed := m.paused != paused
	m.paused = paused
	m.mu.Unlock()
	if changed {
		m.pushPause()
	}
}

// WatchBattery gives the manager a way to tell whether the computer is unplugged; call it before Run.
func (m *Manager) WatchBattery(onBattery func() bool) { m.onBattery = onBattery }

// SetPauseOnBattery says whether indexing waits while the computer runs on its battery.
func (m *Manager) SetPauseOnBattery(on bool) {
	m.mu.Lock()
	m.holdOnBattery = on
	m.mu.Unlock()
	m.checkPower()
}

// watchPower looks at the power source until ctx ends; plugging in or unplugging is noticed within a few seconds.
func (m *Manager) watchPower(ctx context.Context) {
	for {
		m.checkPower()
		select {
		case <-ctx.Done():
			return
		case <-time.After(powerPoll):
		}
	}
}

const powerPoll = 5 * time.Second

func (m *Manager) checkPower() {
	if m.onBattery == nil {
		return
	}
	hold := m.onBattery()
	m.mu.Lock()
	hold = hold && m.holdOnBattery
	changed := m.held != hold
	m.held = hold
	m.mu.Unlock()
	if changed {
		m.pushPause()
	}
}

// pushPause tells the engine whether indexing is stopped (by the person or by the battery) and reports the change.
func (m *Manager) pushPause() {
	stopped := m.isPaused()
	m.tell(func(ctx context.Context) error {
		if stopped {
			return m.engine.Pause(ctx)
		}
		return m.engine.Resume(ctx)
	})
	if m.onChange != nil {
		m.onChange(m.Status())
	}
}

// tell sends a pace request to the engine now, if it is ready; otherwise the next sync does it.
func (m *Manager) tell(request func(ctx context.Context) error) {
	if !m.ready() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := request(ctx); err != nil {
		log.Printf("indexing: telling the engine the pace: %v", err)
	}
}

// applyPace makes the engine match what was asked: its speed and whether it is paused.
func (m *Manager) applyPace(ctx context.Context) {
	m.mu.Lock()
	pace, paused := m.pace, m.paused || m.held
	m.mu.Unlock()
	if err := m.engine.SetDuty(ctx, Duty(pace)); err != nil {
		log.Printf("indexing: setting the speed: %v", err)
	}
	var err error
	if paused {
		err = m.engine.Pause(ctx)
	} else {
		err = m.engine.Resume(ctx)
	}
	if err != nil {
		log.Printf("indexing: setting the pause: %v", err)
	}
}
