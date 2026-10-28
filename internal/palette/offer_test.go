package palette

import (
	"errors"
	"strings"
	"testing"
)

type fakeSound struct {
	files, seconds int
	ok             bool
	on, notNow     int
	failOn         error
}

func (f *fakeSound) Offer() (int, int, bool) { return f.files, f.seconds, f.ok }
func (f *fakeSound) TurnOn() error           { f.on++; return f.failOn }
func (f *fakeSound) NotNow() error           { f.notNow++; return nil }

func withSound(t *testing.T, sound *fakeSound) (*Service, *int) {
	s, dismissed := newService(t)
	With(s, Extras{Sound: sound})
	return s, dismissed
}

func hasOffer(items []Item) bool {
	for _, item := range items {
		if item.ID == "offer:sound" {
			return true
		}
	}
	return false
}

func TestTheOfferComesAfterAFileSearchOfThreeLettersOrMoreAndNamesWhatItCosts(t *testing.T) {
	s, _ := withSound(t, &fakeSound{files: 1204, seconds: 372, ok: true})
	items := s.Query("sleep", ModeFiles).Items
	if len(items) != 1 {
		t.Fatalf("items = %+v", items)
	}
	got := items[0]
	if got.ID != "offer:sound" || got.Kind != "offer" || !strings.Contains(got.Title, "1,204 audio and video files") ||
		!strings.Contains(got.Subtitle, "About 6 min") || len(got.Actions) != 1 || got.Actions[0] != "delete" {
		t.Fatalf("offer = %+v", got)
	}
	if n := len(s.Query("sl", ModeFiles).Items); n != 0 {
		t.Fatalf("two letters is a person still typing, got %d rows", n)
	}
	if n := len(s.Query("", ModeFiles).Items); n != 0 {
		t.Fatalf("nothing typed, got %d rows", n)
	}
}

func TestTheOfferIsOnlyInTheFilesModeOrASoundSearch(t *testing.T) {
	s, _ := withSound(t, &fakeSound{files: 3, seconds: 60, ok: true})
	for _, typed := range []string{"aud: a talk", "sound:rain", "AUDIO : meeting", "audio:"} {
		if !hasOffer(s.Query(typed, ModeAll).Items) {
			t.Errorf("%q: a search for sound carried no offer", typed)
		}
	}
	for _, typed := range []string{"sleep", "chrome", "code: retry"} {
		if hasOffer(s.Query(typed, ModeAll).Items) {
			t.Errorf("%q: the offer showed in an ordinary search", typed)
		}
	}
	if hasOffer(s.Query("sleep", ModeApps).Items) {
		t.Error("the Apps mode carried the offer")
	}
}

func TestNoOfferWhenThereIsNothingToOfferOrNoWayToMakeIt(t *testing.T) {
	s, _ := withSound(t, &fakeSound{ok: false})
	if items := s.Query("sleep", ModeFiles).Items; len(items) != 0 {
		t.Fatalf("items = %+v", items)
	}
	bare, _ := newService(t)
	if items := bare.Query("sleep", ModeFiles).Items; len(items) != 0 {
		t.Fatalf("a bar without the extra showed %+v", items)
	}
}

func TestEnterTurnsTheOfferOnAndClosesTheBar(t *testing.T) {
	sound := &fakeSound{ok: true}
	s, dismissed := withSound(t, sound)
	if err := s.Do("offer:sound", "open", "sleep"); err != nil {
		t.Fatal(err)
	}
	if sound.on != 1 || *dismissed != 1 {
		t.Fatalf("turned on %d times, dismissed %d", sound.on, *dismissed)
	}
}

func TestNotNowIsRememberedAndLeavesTheBarOpen(t *testing.T) {
	sound := &fakeSound{ok: true}
	s, dismissed := withSound(t, sound)
	if err := s.Do("offer:sound", "delete", ""); err != nil {
		t.Fatal(err)
	}
	if sound.notNow != 1 || sound.on != 0 || *dismissed != 0 {
		t.Fatalf("not now %d, on %d, dismissed %d", sound.notNow, sound.on, *dismissed)
	}
}

func TestAFailureToTurnItOnIsShownAndTheBarStaysOpen(t *testing.T) {
	sound := &fakeSound{ok: true, failOn: errors.New("the settings file is read-only")}
	s, dismissed := withSound(t, sound)
	err := s.Do("offer:sound", "open", "")
	if err == nil || !strings.Contains(err.Error(), "read-only") || *dismissed != 0 {
		t.Fatalf("err = %v, dismissed %d", err, *dismissed)
	}
	if err := s.Do("offer:sound", "reveal", ""); err == nil {
		t.Fatal("an offer can only be turned on or set aside")
	}
}

func TestDurationsAreSaidToTheMinuteOrTheHour(t *testing.T) {
	for seconds, want := range map[int]string{20: "under a minute", 44: "under a minute", 45: "1 min", 372: "6 min", 3540: "59 min", 3600: "1 h 0 min", 8100: "2 h 15 min"} {
		if got := duration(seconds); got != want {
			t.Errorf("duration(%d) = %q, want %q", seconds, got, want)
		}
	}
	if plural(1, "audio and video file") != "1 audio and video file" || plural(1204, "file") != "1,204 files" {
		t.Error("the count or its plural is wrong")
	}
}
