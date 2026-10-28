// Package soundoffer decides when the bar offers to search the sound of audio files and videos, and turns that on.
// The offer is for someone who has file search but left the choice at file names (docs/superpowers/specs/
// 2026-10-28-audio-search-design.md): it appears next to a file search, says what it would cost for their own folders,
// and "not now" is remembered for a month.
package soundoffer

import (
	"fmt"
	"time"

	"omni/internal/config"
	"omni/internal/estimate"
	"omni/internal/sidecar"
	"omni/internal/survey"
)

// ID names the offer wherever it is remembered.
const ID = "offer:sound"

// Quiet is how long "not now" is honoured.
const Quiet = 30 * 24 * time.Hour

// Settings is the settings store; *settings.Store satisfies it.
type Settings interface {
	Get() config.Config
	Update(change func(*config.Config)) error
}

// Surveys is what the folders hold; *survey.Manager satisfies it.
type Surveys interface {
	Results() map[string]survey.Result
}

// Waivers remembers "not now"; *state.Store satisfies it.
type Waivers interface {
	Waived(id string) time.Time
	Waive(id string, at time.Time) error
}

// Service makes the offer.
type Service struct {
	Settings Settings
	Surveys  Surveys
	Waivers  Waivers
	// Ready says whether the engine is up and as new as this launcher: an older one does not know the choice and would
	// refuse the next sync.
	Ready func() bool
	// OnCPU says whether the engine runs on the processor, where sound is slower.
	OnCPU func() bool
	Now   func() time.Time
}

// Offer says how many audio files and videos there are to search by sound and about how many seconds it would take on
// this computer, or false when there is nothing to offer: file search off, the choice already made, the engine not
// ready or out of date, no sound in the folders, or a "not now" less than a month old.
func (s *Service) Offer() (files, seconds int, ok bool) {
	cfg := s.Settings.Get()
	if !cfg.Search.Enabled || cfg.Search.Audio != "name" || !s.Ready() {
		return 0, 0, false
	}
	if at := s.Waivers.Waived(ID); !at.IsZero() && s.Now().Sub(at) < Quiet {
		return 0, 0, false
	}
	name := options(cfg.Search, "name")
	content := options(cfg.Search, "content")
	results := s.Surveys.Results()
	var extra time.Duration
	for _, root := range cfg.Roots {
		result := results[root]
		if result.State != survey.Done {
			continue
		}
		routes := result.Survey.Routes
		files += routes["audio"].Files + routes["video"].Files
		with := estimate.WorkFor(routes, content).Duration(cfg.Search.Pace, s.OnCPU())
		without := estimate.WorkFor(routes, name).Duration(cfg.Search.Pace, s.OnCPU())
		extra += with - without
	}
	if files == 0 {
		return 0, 0, false
	}
	return files, int(extra.Seconds() + 0.5), true
}

func options(s config.Search, audio string) sidecar.IndexOptions {
	return sidecar.IndexOptions{PDFPages: s.PDFPages, Photos: s.Photos, Videos: s.Videos, Audio: audio}
}

// TurnOn makes the choice: the settings subscribers start the sync that indexes the sound.
func (s *Service) TurnOn() error {
	return s.Settings.Update(func(c *config.Config) { c.Search.Audio = "content" })
}

// NotNow remembers that the person does not want the offer for a while.
func (s *Service) NotNow() error {
	if err := s.Waivers.Waive(ID, s.Now()); err != nil {
		return fmt.Errorf("remember not now: %w", err)
	}
	return nil
}
