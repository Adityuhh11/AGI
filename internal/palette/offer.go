package palette

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"omni/internal/soundoffer"
)

// SoundOffer is the offer to search the sound of audio files and videos; *soundoffer.Service satisfies it. Offer says
// whether to make it and what it would cost for this person's folders.
type SoundOffer interface {
	Offer() (files, seconds int, ok bool)
	TurnOn() error
	NotNow() error
}

// offerMinLength is the shortest search that carries the offer: two letters is a person still typing.
const offerMinLength = 3

// soundScope is a search that asks for sound by name (`aud:`, `audio:`, `sound:`); search/scope.go has the same words.
var soundScope = regexp.MustCompile(`(?i)^\s*(aud|audio|sounds?)\s*:`)

// offerItems is the offer as a row when a file search is under way, or none. The row sits after the files (Kind "offer"), so
// a search for a file is answered first.
func (s *Service) offerItems(raw, text string) []Item {
	if s.x.Sound == nil || (utf8.RuneCountInString(text) < offerMinLength && !soundScope.MatchString(raw)) {
		return nil
	}
	files, seconds, ok := s.x.Sound.Offer()
	if !ok {
		return nil
	}
	return []Item{{
		ID:       soundoffer.ID,
		Kind:     "offer",
		Title:    fmt.Sprintf("Also search what is said and played in your %s?", plural(files, "audio and video file")),
		Subtitle: "About " + duration(seconds) + " on this computer. Enter turns it on; Ctrl+Delete for not now.",
		Actions:  []string{"delete"},
	}}
}

// doOffer turns the offer on ("open") or sets it aside ("delete"), and closes the bar: the sync that follows is shown by the
// tray and by Settings.
func (s *Service) doOffer(action string) error {
	if s.x.Sound == nil {
		return fmt.Errorf("there is nothing to turn on")
	}
	switch action {
	case "open":
		if err := s.x.Sound.TurnOn(); err != nil {
			return fmt.Errorf("turn on sound search: %w", err)
		}
		s.dismiss()
		return nil
	case "delete":
		return s.x.Sound.NotNow()
	}
	return fmt.Errorf("an offer can be turned on or set aside, not %q", action)
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%s %ss", groupDigits(n), noun)
}

// groupDigits writes 1204 as 1,204.
func groupDigits(n int) string {
	digits := fmt.Sprint(n)
	var out strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(d)
	}
	return out.String()
}

// duration says seconds the way the setup guide does: to the minute, or the hour and the minute.
func duration(seconds int) string {
	switch minutes := (seconds + 30) / 60; {
	case seconds < 45:
		return "under a minute"
	case minutes < 60:
		return fmt.Sprintf("%d min", minutes)
	default:
		return fmt.Sprintf("%d h %d min", minutes/60, minutes%60)
	}
}
