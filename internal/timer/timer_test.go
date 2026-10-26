package timer

import (
	"context"
	"testing"
	"time"

	"agi/internal/config"
)

// clock is a time the test moves by hand.
type clock struct{ t time.Time }

func (c *clock) now() time.Time       { return c.t }
func (c *clock) pass(d time.Duration) { c.t = c.t.Add(d) }

type harness struct {
	*Timer
	clock    *clock
	cfg      config.Timer
	finished []State
	active   []bool
}

func newHarness() *harness {
	h := &harness{clock: &clock{t: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)}, cfg: config.Default().Timer}
	h.Timer = New(func() config.Timer { return h.cfg }, func(s State) { h.finished = append(h.finished, s) }, func(a bool) { h.active = append(h.active, a) })
	h.Timer.now = h.clock.now
	return h
}

func (h *harness) pass(d time.Duration) { h.clock.pass(d); h.advance() }

func TestAPlainTimerCountsDownAndFinishesOnce(t *testing.T) {
	h := newHarness()
	h.Start(90*time.Second, "tea")
	if s := h.State(); !s.Active || !s.Running || s.Remaining != 90 || s.Total != 90 || s.Label != "tea" || s.Phase != Countdown || s.Rounds != 0 {
		t.Fatalf("state = %+v", s)
	}
	h.pass(30*time.Second + 400*time.Millisecond)
	if s := h.State(); s.Remaining != 60 {
		t.Errorf("remaining = %d, want 60 (rounded up)", s.Remaining)
	}
	h.pass(59 * time.Second)
	if len(h.finished) != 0 || h.State().Remaining != 1 {
		t.Fatalf("finished early: %v, %+v", h.finished, h.State())
	}
	h.pass(2 * time.Second)
	h.pass(10 * time.Second) // nothing more happens
	s := h.State()
	if !s.Finished || s.Running || s.Remaining != 0 || len(h.finished) != 1 || h.finished[0].Label != "tea" {
		t.Fatalf("state = %+v, finished %v", s, h.finished)
	}
	if s.Next != "" {
		t.Errorf("a plain timer has nothing after it: %q", s.Next)
	}
}

func TestPausingStopsTheClockAndResumingContinuesFromWhereItWas(t *testing.T) {
	h := newHarness()
	h.Start(time.Minute, "")
	h.pass(20 * time.Second)
	h.Pause()
	h.pass(10 * time.Minute) // a long time passes while paused
	if s := h.State(); !s.Paused || s.Running || s.Remaining != 40 || s.Finished || len(h.finished) != 0 {
		t.Fatalf("paused state = %+v", s)
	}
	h.Resume()
	h.pass(39 * time.Second)
	if s := h.State(); s.Remaining != 1 || s.Paused {
		t.Fatalf("resumed state = %+v", s)
	}
	h.pass(time.Second)
	if !h.State().Finished {
		t.Error("did not finish after the rest of the time")
	}
}

func TestPauseAndResumeDoNothingWhenThereIsNothingToPauseOrResume(t *testing.T) {
	h := newHarness()
	h.Pause()
	h.Resume()
	if h.State().Active {
		t.Error("pausing nothing made a timer")
	}
	h.Start(time.Minute, "")
	h.Resume() // not paused
	h.pass(time.Minute)
	h.Pause() // finished
	if s := h.State(); s.Paused || !s.Finished {
		t.Errorf("state = %+v", s)
	}
}

func TestStopEndsEverythingAndTellsTheWindow(t *testing.T) {
	h := newHarness()
	h.Stop() // nothing to stop: no news
	h.Start(time.Minute, "")
	h.Start(2*time.Minute, "again") // replaces, and the window is already open
	h.Stop()
	h.Stop()
	if got := h.State(); got != (State{}) {
		t.Errorf("state = %+v", got)
	}
	if len(h.active) != 2 || !h.active[0] || h.active[1] {
		t.Errorf("window told %v, want open once and close once", h.active)
	}
}

func TestANonsenseStartIsIgnored(t *testing.T) {
	h := newHarness()
	h.Start(0, "x")
	h.Start(-time.Second, "x")
	if h.State().Active {
		t.Error("a timer of nothing started")
	}
}

func TestAPomodoroGoesFocusBreakFocusAndEndsWithTheLongBreak(t *testing.T) {
	h := newHarness()
	h.cfg = config.Timer{FocusMinutes: 25, ShortBreakMinutes: 5, LongBreakMinutes: 15, Rounds: 2}
	h.StartPomodoro()
	want := []struct {
		phase     Phase
		round     int
		remaining int
		next      string
	}{
		{Focus, 1, 25 * 60, "Short break"},
		{ShortBreak, 1, 5 * 60, "Focus"},
		{Focus, 2, 25 * 60, "Long break"},
		{LongBreak, 2, 15 * 60, ""},
	}
	for i, w := range want {
		s := h.State()
		if s.Phase != w.phase || s.Round != w.round || s.Rounds != 2 || s.Remaining != w.remaining || s.Next != w.next || !s.Running {
			t.Fatalf("period %d: %+v, want %+v", i, s, w)
		}
		h.pass(time.Duration(w.remaining) * time.Second)
		if s := h.State(); !s.Finished || s.Running {
			t.Fatalf("period %d did not wait for a click: %+v", i, s)
		}
		if i < len(want)-1 {
			h.Skip() // the click on "next"
		}
	}
	h.Skip() // after the long break there is nothing to go on to: the timer is dismissed
	if h.State().Active || len(h.finished) != 4 {
		t.Errorf("state %+v, finished %d times", h.State(), len(h.finished))
	}
}

func TestWithAutoNextThePomodoroCarriesOnByItself(t *testing.T) {
	h := newHarness()
	h.cfg = config.Timer{FocusMinutes: 1, ShortBreakMinutes: 1, LongBreakMinutes: 1, Rounds: 2, AutoNext: true}
	h.StartPomodoro()
	h.pass(time.Minute)
	if s := h.State(); s.Phase != ShortBreak || !s.Running || s.Finished || len(h.finished) != 1 {
		t.Fatalf("state = %+v", s)
	}
	h.pass(time.Minute)
	h.pass(time.Minute)
	if s := h.State(); s.Phase != LongBreak || !s.Running {
		t.Fatalf("state = %+v", s)
	}
	h.pass(time.Minute) // the long break ends the cycle: nothing follows, so it waits
	if s := h.State(); !s.Finished || s.Phase != LongBreak {
		t.Fatalf("state = %+v", s)
	}
}

func TestSkippingJumpsToTheNextPeriodAtOnceAndStopsAPlainTimer(t *testing.T) {
	h := newHarness()
	h.StartPomodoro()
	h.pass(time.Minute)
	h.Skip()
	if s := h.State(); s.Phase != ShortBreak || s.Remaining != 5*60 || !s.Running {
		t.Fatalf("state = %+v", s)
	}
	h.Start(time.Minute, "")
	h.Skip()
	if h.State().Active {
		t.Error("skipping a plain timer should end it")
	}
}

func TestChangedSettingsApplyToTheNextPeriod(t *testing.T) {
	h := newHarness()
	h.StartPomodoro()
	h.cfg.ShortBreakMinutes = 10
	h.Skip()
	if h.State().Total != 600 {
		t.Errorf("total = %d, want the new 10-minute break", h.State().Total)
	}
}

func TestRunNoticesTheEndWithoutAnyoneLooking(t *testing.T) {
	h := newHarness()
	h.Start(time.Second, "")
	h.clock.pass(2 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.Run(ctx); close(done) }()
	deadline := time.After(2 * time.Second)
	for !h.State().Finished {
		select {
		case <-deadline:
			t.Fatal("Run never finished the timer")
		case <-time.After(20 * time.Millisecond):
		}
	}
	cancel()
	<-done
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		text  string
		d     time.Duration
		label string
		ok    bool
	}{
		{"25", 25 * time.Minute, "", true},
		{"90s", 90 * time.Second, "", true},
		{"1h30", 90 * time.Minute, "", true},
		{"1h 30m", 90 * time.Minute, "", true},
		{"1.5h", 90 * time.Minute, "", true},
		{"10m tea", 10 * time.Minute, "tea", true},
		{"10 min tea time", 10 * time.Minute, "tea time", true},
		{"5 eggs", 5 * time.Minute, "eggs", true},
		{"1m30", 90 * time.Second, "", true},
		{"2 hours", 2 * time.Hour, "", true},
		{"45 SECONDS", 45 * time.Second, "", true},
		{"0.5", 30 * time.Second, "", true},
		{"", 0, "", false},
		{"tea 10", 0, "", false},
		{"0", 0, "", false},
		{"0s", 0, "", false},
		{"25h", 0, "", false},
		{"1000000000000000000000", 0, "", false},
		{"1:30", 0, "", false},
	}
	for _, tt := range tests {
		d, label, ok := ParseDuration(tt.text)
		if d != tt.d || label != tt.label || ok != tt.ok {
			t.Errorf("ParseDuration(%q) = %v, %q, %v; want %v, %q, %v", tt.text, d, label, ok, tt.d, tt.label, tt.ok)
		}
	}
}
