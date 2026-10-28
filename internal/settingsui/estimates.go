package settingsui

import (
	"omni/internal/config"
	"omni/internal/estimate"
	"omni/internal/sidecar"
	"omni/internal/survey"
)

// What the folders hold and how long indexing them will take on this computer, for the File search page and the setup
// guide. The numbers come from internal/survey (the engine's own count) and internal/estimate (measured rates).

// Surveys is the survey manager as the service uses it; *survey.Manager satisfies it.
type Surveys interface {
	Results() map[string]survey.Result
}

// FolderSurvey is what one folder holds. Seconds is the estimate for indexing it with the choices and the speed now,
// 0 until it is counted.
type FolderSurvey struct {
	Root string
	// State is pending, running, done, failed or unavailable (the engine installed cannot count).
	State   string
	Files   int
	Bytes   int64
	Seconds int
}

// Estimates is what indexing every folder would take, 0 where it cannot be said yet.
type Estimates struct {
	// Complete says every folder is counted, so the totals are the whole job.
	Complete bool
	// Seconds is the total with the choices and the speed now.
	Seconds int
	// ByChoice is the total if one choice were another, the rest staying as they are, keyed "PDFPages:all",
	// "Photos:name" and so on; ByPace the same for the speed, keyed "fast", "balanced" and "gentle".
	ByChoice map[string]int
	ByPace   map[string]int
}

// surveys lists the configured folders with what each holds.
func (s *Service) surveys(cfg config.Config, onCPU bool) []FolderSurvey {
	results := s.o.Surveys.Results()
	out := make([]FolderSurvey, 0, len(cfg.Roots))
	for _, root := range cfg.Roots {
		r := results[root]
		view := FolderSurvey{Root: root, State: string(r.State)}
		if r.State == survey.Done {
			for _, c := range r.Survey.Routes {
				view.Files += c.Files
				view.Bytes += c.Bytes
			}
			view.Seconds = seconds(r.Survey.Routes, indexOptions(cfg.Search), cfg.Search.Pace, onCPU)
		}
		out = append(out, view)
	}
	return out
}

// estimates totals the surveyed folders under the choices now and under each single change of a choice or the speed.
func (s *Service) estimates(cfg config.Config, onCPU bool) Estimates {
	results := s.o.Surveys.Results()
	var all []map[string]sidecar.RouteCount
	complete := len(cfg.Roots) > 0
	for _, root := range cfg.Roots {
		if r := results[root]; r.State == survey.Done {
			all = append(all, r.Survey.Routes)
		} else {
			complete = false
		}
	}
	total := func(o sidecar.IndexOptions, pace string) int {
		sum := 0
		for _, routes := range all {
			sum += seconds(routes, o, pace, onCPU)
		}
		return sum
	}
	now := indexOptions(cfg.Search)
	out := Estimates{Complete: complete, ByChoice: map[string]int{}, ByPace: map[string]int{}}
	if len(all) == 0 {
		return out
	}
	out.Seconds = total(now, cfg.Search.Pace)
	for _, mode := range config.PDFPageModes {
		o := now
		o.PDFPages = mode
		out.ByChoice["PDFPages:"+mode] = total(o, cfg.Search.Pace)
	}
	for _, mode := range config.PhotoModes {
		o := now
		o.Photos = mode
		out.ByChoice["Photos:"+mode] = total(o, cfg.Search.Pace)
	}
	for _, mode := range config.VideoModes {
		o := now
		o.Videos = mode
		out.ByChoice["Videos:"+mode] = total(o, cfg.Search.Pace)
	}
	for _, mode := range config.AudioModes {
		o := now
		o.Audio = mode
		out.ByChoice["Audio:"+mode] = total(o, cfg.Search.Pace)
	}
	for _, pace := range config.Paces {
		out.ByPace[pace] = total(now, pace)
	}
	return out
}

func seconds(routes map[string]sidecar.RouteCount, o sidecar.IndexOptions, pace string, onCPU bool) int {
	return int(estimate.WorkFor(routes, o).Duration(pace, onCPU).Seconds() + 0.5)
}

func indexOptions(s config.Search) sidecar.IndexOptions {
	return sidecar.IndexOptions{PDFPages: s.PDFPages, Photos: s.Photos, Videos: s.Videos, Audio: s.Audio}
}
