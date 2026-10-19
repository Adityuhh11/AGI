package clipboard

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

// scene plays the screen: it records what happened in order, and the program the keyboard goes back to
// becomes the foreground window after comeBackAfter of waiting.
type scene struct {
	events        []string
	target        uintptr
	elevated      bool
	setTextFails  bool
	comeBackAfter time.Duration
	released      bool
	slept         time.Duration
}

func (s *scene) paster() Paster {
	return Paster{
		SetText: func(text string) bool {
			s.events = append(s.events, "set:"+text)
			return !s.setTextFails
		},
		Previous: func() uintptr { return s.target },
		Release:  func() { s.events = append(s.events, "release"); s.released = true },
		Foreground: func() uintptr {
			if s.released && s.slept >= s.comeBackAfter {
				return s.target
			}
			return 1 // the bar, or anything else
		},
		Elevated:  func(uintptr) bool { return s.elevated },
		SendPaste: func() error { s.events = append(s.events, "ctrl+v"); return nil },
		Sleep:     func(d time.Duration) { s.slept += d },
	}
}

func order(s *scene) string { return strings.Join(s.events, ",") }

func TestAPasteSetsTheTextThenGivesTheKeyboardBackThenTypesCtrlV(t *testing.T) {
	s := &scene{target: 42, comeBackAfter: 20 * time.Millisecond}
	if err := s.paster().Paste("hello"); err != nil {
		t.Fatal(err)
	}
	if got := order(s); got != "set:hello,release,ctrl+v" {
		t.Fatalf("events %q", got)
	}
	if s.slept < 20*time.Millisecond+settle {
		t.Errorf("slept %v: it must wait for the program and then let it settle", s.slept)
	}
}

func TestNothingIsTypedIntoAProgramWindowsWillNotLetUsTypeInto(t *testing.T) {
	s := &scene{target: 42, elevated: true}
	err := s.paster().Paste("secret")
	if !errors.Is(err, ErrElevated) {
		t.Fatalf("err = %v", err)
	}
	if got := order(s); got != "set:secret" {
		t.Fatalf("events %q: the text is copied, the bar stays open (no release) and nothing is typed", got)
	}
}

func TestWithNoProgramToGoBackToTheTextIsCopiedAndTheBarStaysOpen(t *testing.T) {
	s := &scene{target: 0}
	if err := s.paster().Paste("x"); !errors.Is(err, ErrNoTarget) {
		t.Fatalf("err = %v", err)
	}
	if got := order(s); got != "set:x" {
		t.Fatalf("events %q", got)
	}
}

func TestAProgramThatNeverComesBackGetsNoKeysAndTheErrorSaysWhyTheTextIsOnTheClipboard(t *testing.T) {
	s := &scene{target: 42, comeBackAfter: time.Hour}
	err := s.paster().Paste("x")
	if err == nil || !strings.Contains(err.Error(), "on the clipboard") {
		t.Fatalf("err = %v", err)
	}
	if slices.Contains(s.events, "ctrl+v") {
		t.Fatal("keys were typed into a window that never became the foreground window")
	}
	if s.slept > foregroundWait+10*time.Millisecond {
		t.Errorf("waited %v, the limit is %v", s.slept, foregroundWait)
	}
}

func TestAClipboardThatCannotBeWrittenStopsBeforeAnythingIsReleasedOrTyped(t *testing.T) {
	s := &scene{target: 42, setTextFails: true}
	if err := s.paster().Paste("x"); err == nil {
		t.Fatal("no error")
	}
	if got := order(s); got != "set:x" {
		t.Fatalf("events %q", got)
	}
}

func TestACopyGivesTheKeyboardBackAndTypesNothing(t *testing.T) {
	s := &scene{target: 42}
	if err := s.paster().Copy("abc"); err != nil {
		t.Fatal(err)
	}
	if got := order(s); got != "set:abc,release" {
		t.Fatalf("events %q", got)
	}
}
