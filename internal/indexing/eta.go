package indexing

import (
	"fmt"
	"time"
)

// An estimate needs a few files done and a few seconds worked, or the first slow file would promise hours.
const (
	minFilesForEstimate = 5
	minWorkForEstimate  = 5 * time.Second
)

// rateWindow is how much recent work the estimate looks at: long enough to smooth over one big file, short enough that a
// new speed shows within a minute or two instead of being averaged away by everything done at the old one.
const rateWindow = 90 * time.Second

// minFilesInWindow is how many files the window must have seen finish before its rate is believed.
const minFilesInWindow = 3

// progress is how far a job had got after it had worked this long (a pause does not count as work).
type progress struct {
	worked time.Duration
	done   int
}

// estimator says how long a job has left. It uses the rate of the last rateWindow of work, and the average since the
// speed last changed (restart) while the window is too thin to say; what was done before the change says nothing about
// what is left, because the speed decides how fast files go by.
type estimator struct {
	origin progress   // the job's start, or the moment the speed changed
	recent []progress // samples inside the window, plus the newest one before it
}

// restart forgets the past: the speed changed, so the rate so far no longer applies.
func (e *estimator) restart(at progress) { *e = estimator{origin: at} }

// remaining takes the latest progress of a job of total files and returns the time left, rounded to the minute; 0 means
// "no estimate yet", because the first slow file would otherwise promise hours.
func (e *estimator) remaining(now progress, total int) time.Duration {
	e.recent = append(e.recent, now)
	for len(e.recent) > 1 && now.worked-e.recent[1].worked >= rateWindow {
		e.recent = e.recent[1:]
	}
	if now.done >= total {
		return 0
	}
	if from := e.recent[0]; now.done-from.done >= minFilesInWindow && now.worked-from.worked >= minWorkForEstimate {
		return roundedLeft(now.worked-from.worked, now.done-from.done, total-now.done)
	}
	if now.done-e.origin.done >= minFilesForEstimate && now.worked-e.origin.worked >= minWorkForEstimate {
		return roundedLeft(now.worked-e.origin.worked, now.done-e.origin.done, total-now.done)
	}
	return 0
}

// roundedLeft is the time for left more files at the rate of done files in worked. Files differ a lot in cost, so it is
// rounded to the minute and never promises less than one.
func roundedLeft(worked time.Duration, done, left int) time.Duration {
	return max(time.Duration(float64(worked)/float64(done)*float64(left)).Round(time.Minute), time.Minute)
}

// LeftText is the end of a status sentence ("about 10 min left"), or "" when there is no estimate.
func LeftText(left time.Duration) string {
	switch {
	case left <= 0:
		return ""
	case left <= time.Minute:
		return ", less than a minute left"
	case left < 90*time.Minute:
		return fmt.Sprintf(", about %d min left", int(left.Minutes()))
	default:
		return fmt.Sprintf(", about %d h left", int(left.Round(time.Hour).Hours()))
	}
}
