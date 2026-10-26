// Package timer is the countdown and the pomodoro. It lives in the program, not in its window, so a timer keeps running
// when the window is closed and the window only shows it. The clock is injected, so the whole thing is tested without waiting.
package timer

import (
	"context"
	"sync"
	"time"

	"agi/internal/config"
)

// Phase is what a period of a pomodoro is.
type Phase string

const (
	Countdown  Phase = "countdown" // a plain timer, not part of a pomodoro
	Focus      Phase = "focus"
	ShortBreak Phase = "short"
	LongBreak  Phase = "long"
)

// State is the timer as the window shows it.
type State struct {
	// Active is true from the start until the timer is stopped or its last period has been dismissed.
	Active bool
	// Running is true while time passes; a paused or finished timer is not running.
	Running bool
	Paused  bool
	// Finished says the period reached zero and waits to be dismissed or followed by the next one.
	Finished bool
	Phase    Phase
	// Label is what the period is called: the words typed after the time ("tea"), or Focus, Short break, Long break.
	Label string
	// Total and Remaining are in seconds; Remaining is rounded up, so a timer shows 0 only when it is done.
	Total, Remaining int
	// Round and Rounds say where a pomodoro is ("2 of 4"); both are 0 for a plain timer.
	Round, Rounds int
	// Next names the period that follows in a pomodoro, or is empty.
	Next string
}

// Timer holds at most one timer.
type Timer struct {
	now      func() time.Time
	settings func() config.Timer
	onFinish func(State)
	onActive func(bool)

	// notify makes the calls to onActive one at a time, each with what the timer is at that moment: whatever order two
	// changes arrive in, the window ends up as the last state says.
	notify sync.Mutex

	mu       sync.Mutex
	gen      int // changes whenever a period starts or the timer stops, so a decision made earlier can tell it is out of date
	active   bool
	paused   bool
	finished bool
	phase    Phase
	label    string
	round    int
	rounds   int
	total    time.Duration
	deadline time.Time     // when the running period ends
	left     time.Duration // what was left when it was paused
}

// New returns a timer. settings gives the pomodoro's lengths each time they are needed (so a change in Settings counts at once);
// onFinish is called when a period ends (the beep) and onActive with whether there is a timer, each time one starts or ends (the
// window opens and closes; it must cope with being told the same thing twice). Either may be nil.
func New(settings func() config.Timer, onFinish func(State), onActive func(bool)) *Timer {
	return &Timer{now: time.Now, settings: settings, onFinish: onFinish, onActive: onActive}
}

// Run advances the timer until ctx ends: it notices a period that has run out even when nobody is looking at it.
func (t *Timer) Run(ctx context.Context) {
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			t.advance()
		}
	}
}

// Start begins a plain countdown of d, replacing any timer there was.
func (t *Timer) Start(d time.Duration, label string) {
	if d <= 0 {
		return
	}
	if label == "" {
		label = "Timer"
	}
	t.begin(Countdown, label, d, 0, 0)
}

// StartPomodoro begins the first focus period of a pomodoro, replacing any timer there was.
func (t *Timer) StartPomodoro() {
	c := t.settings()
	t.begin(Focus, "Focus", minutes(c.FocusMinutes), 1, c.Rounds)
}

func minutes(n int) time.Duration { return time.Duration(n) * time.Minute }

func (t *Timer) begin(phase Phase, label string, d time.Duration, round, rounds int) {
	t.mu.Lock()
	t.startLocked(phase, label, d, round, rounds)
	t.mu.Unlock()
	t.sync()
}

// startLocked makes a period the current one and starts its clock. The deadline drops Go's monotonic reading (Round(0)): that
// clock may not count the time a computer spends asleep, and a 25-minute timer should end 25 minutes after it began on the wall.
func (t *Timer) startLocked(phase Phase, label string, d time.Duration, round, rounds int) {
	t.gen++
	t.active, t.paused, t.finished = true, false, false
	t.phase, t.label, t.total, t.round, t.rounds = phase, label, d, round, rounds
	t.deadline = t.now().Add(d).Round(0)
	t.left = 0
}

// sync tells the window whether there is a timer, as it is now.
func (t *Timer) sync() {
	if t.onActive == nil {
		return
	}
	t.notify.Lock()
	defer t.notify.Unlock()
	t.onActive(t.State().Active)
}

// Pause stops the clock of a running period; Resume starts it again. Neither does anything else.
func (t *Timer) Pause() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.active && !t.paused && !t.finished {
		t.left = max(0, t.deadline.Sub(t.now()))
		t.paused = true
	}
}

// Resume continues a paused period.
func (t *Timer) Resume() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.active && t.paused {
		t.deadline = t.now().Add(t.left).Round(0)
		t.paused = false
	}
}

// Stop ends the timer, whatever it was doing.
func (t *Timer) Stop() {
	t.mu.Lock()
	was := t.active
	t.gen++
	t.active, t.paused, t.finished = false, false, false
	t.mu.Unlock()
	if was {
		t.sync()
	}
}

// Skip moves a pomodoro on to its next period and starts it; on a plain timer, or at the end of a pomodoro, it stops the timer.
func (t *Timer) Skip() {
	t.mu.Lock()
	if !t.active {
		t.mu.Unlock()
		return
	}
	phase, label, d, round, ok := t.followingLocked()
	if ok {
		t.startLocked(phase, label, d, round, t.rounds)
	}
	t.mu.Unlock()
	if ok {
		t.sync()
	} else {
		t.Stop()
	}
}

// followingLocked is the period after the current one in a pomodoro: focus is followed by a short break, or by the long one
// after the last round; a short break by the next focus; the long break by nothing.
func (t *Timer) followingLocked() (phase Phase, label string, d time.Duration, round int, ok bool) {
	c := t.settings()
	switch t.phase {
	case Focus:
		if t.round < t.rounds {
			return ShortBreak, "Short break", minutes(c.ShortBreakMinutes), t.round, true
		}
		return LongBreak, "Long break", minutes(c.LongBreakMinutes), t.round, true
	case ShortBreak:
		return Focus, "Focus", minutes(c.FocusMinutes), t.round + 1, true
	}
	return "", "", 0, 0, false
}

// advance ends a period whose time has run out: the next one starts by itself when settings say so, otherwise the timer
// waits, finished, to be dismissed or skipped on. If the timer is stopped or restarted while the finish is being announced, the
// next period is not started over that.
func (t *Timer) advance() {
	t.mu.Lock()
	if !t.active || t.paused || t.finished || t.now().Before(t.deadline) {
		t.mu.Unlock()
		return
	}
	t.finished = true
	finished := t.stateLocked()
	gen := t.gen
	phase, label, d, round, more := t.followingLocked()
	auto := more && t.settings().AutoNext
	t.mu.Unlock()
	if t.onFinish != nil {
		t.onFinish(finished)
	}
	if !auto {
		return
	}
	t.mu.Lock()
	if t.gen != gen {
		t.mu.Unlock()
		return
	}
	t.startLocked(phase, label, d, round, finished.Rounds)
	t.mu.Unlock()
	t.sync()
}

// State is the timer now.
func (t *Timer) State() State {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stateLocked()
}

func (t *Timer) stateLocked() State {
	if !t.active {
		return State{}
	}
	left := t.left
	if !t.paused {
		left = max(0, t.deadline.Sub(t.now()))
	}
	if t.finished {
		left = 0
	}
	s := State{
		Active: true, Running: !t.paused && !t.finished, Paused: t.paused, Finished: t.finished,
		Phase: t.phase, Label: t.label, Total: int(t.total / time.Second), Remaining: int((left + time.Second - 1) / time.Second),
		Round: t.round, Rounds: t.rounds,
	}
	if _, label, _, _, ok := t.followingLocked(); ok {
		s.Next = label
	}
	return s
}

// Settings is the pomodoro's lengths now (for the bar's rows).
func (t *Timer) Settings() config.Timer { return t.settings() }
