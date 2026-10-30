package indexing

import (
	"testing"
	"time"

	"omni/internal/testpath"
)

func remaining(worked time.Duration, done, total int) time.Duration {
	var e estimator
	return e.remaining(progress{worked, done}, total)
}

func TestEstimatorFollowsTheSpeedOfNowNotTheAverage(t *testing.T) {
	var e estimator
	// Gentle: a file every 10 s for 10 minutes, then Fast: a file a second.
	at := progress{}
	var left time.Duration
	for i := 0; i < 60; i++ {
		at = progress{at.worked + 10*time.Second, at.done + 1}
		e.remaining(at, 1000)
	}
	e.restart(at)
	for i := 0; i < 120; i++ { // two minutes of Fast
		at = progress{at.worked + time.Second, at.done + 1}
		left = e.remaining(at, 1000)
	}
	// 820 files left at one a second is about 14 minutes; the average over the whole job would say over 2 hours.
	if want := 14 * time.Minute; left != want {
		t.Errorf("left after the speed went up = %v, want %v", left, want)
	}
}

func TestEstimatorSaysNothingJustAfterTheSpeedChanges(t *testing.T) {
	var e estimator
	at := progress{5 * time.Minute, 30}
	e.remaining(at, 1000)
	e.restart(at)
	at = progress{at.worked + 2*time.Second, at.done + 2}
	if left := e.remaining(at, 1000); left != 0 {
		t.Errorf("left 2 s after a change = %v, want no estimate", left)
	}
}

func TestRemainingNeedsEnoughWorkToBeWorthSaying(t *testing.T) {
	tests := []struct {
		name        string
		worked      time.Duration
		done, total int
		want        time.Duration
	}{
		{"too few files", 20 * time.Second, 3, 100, 0},
		{"too little time", 2 * time.Second, 10, 100, 0},
		{"finished", time.Minute, 100, 100, 0},
		{"a tenth in 1 min of work", time.Minute, 10, 100, 9 * time.Minute},
		{"rounded to the minute", 100 * time.Second, 10, 100, 15 * time.Minute},
		{"never less than a minute", 10 * time.Second, 99, 100, time.Minute},
	}
	for _, tt := range tests {
		if got := remaining(tt.worked, tt.done, tt.total); got != tt.want {
			t.Errorf("%s: remaining = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestLeftTextSaysMinutesThenHours(t *testing.T) {
	for left, want := range map[time.Duration]string{
		0:                         "",
		time.Minute:               ", less than a minute left",
		10 * time.Minute:          ", about 10 min left",
		89 * time.Minute:          ", about 89 min left",
		2*time.Hour + time.Minute: ", about 2 h left",
		5 * time.Hour:             ", about 5 h left",
	} {
		if got := LeftText(left); got != want {
			t.Errorf("LeftText(%v) = %q, want %q", left, got, want)
		}
	}
	got := Status{State: Indexing, Root: testpath.P(`C:\Docs`), FilesDone: 56, FilesTotal: 922, Remaining: 10 * time.Minute}.String()
	if want := "Indexing Docs: 56 / 922 files, about 10 min left"; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
}
