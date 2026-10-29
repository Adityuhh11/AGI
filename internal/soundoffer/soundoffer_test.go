package soundoffer

import (
	"errors"
	"testing"
	"time"

	"omni/internal/config"
	"omni/internal/sidecar"
	"omni/internal/survey"
	"omni/internal/testpath"
)

type fakeSettings struct{ cfg config.Config }

func (f *fakeSettings) Get() config.Config { return f.cfg }
func (f *fakeSettings) Update(change func(*config.Config)) error {
	change(&f.cfg)
	return nil
}

type fakeSurveys map[string]survey.Result

func (f fakeSurveys) Results() map[string]survey.Result { return f }

type fakeWaivers struct {
	at  time.Time
	err error
}

func (f *fakeWaivers) Waived(string) time.Time { return f.at }
func (f *fakeWaivers) Waive(_ string, at time.Time) error {
	f.at = at
	return f.err
}

var now = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

func world() (*Service, *fakeSettings, *fakeWaivers) {
	cfg := config.Default()
	cfg.Roots = []string{testpath.P(`C:\Media`)}
	cfg.Search.Enabled = true
	settings := &fakeSettings{cfg: cfg}
	waivers := &fakeWaivers{}
	routes := map[string]sidecar.RouteCount{
		"audio": {Files: 200, Bytes: 1 << 30, Seconds: 3600 * 20},
		"video": {Files: 12, Bytes: 1 << 30, Seconds: 3600},
		"code":  {Files: 100, Bytes: 1 << 20},
	}
	return &Service{
		Settings: settings,
		Surveys:  fakeSurveys{testpath.P(`C:\Media`): {State: survey.Done, Survey: sidecar.Survey{Routes: routes}}},
		Waivers:  waivers,
		Ready:    func() bool { return true },
		OnCPU:    func() bool { return false },
		Now:      func() time.Time { return now },
	}, settings, waivers
}

func TestTheOfferCountsAudioAndVideosAndPricesTheSoundOnThisComputer(t *testing.T) {
	s, _, _ := world()
	files, seconds, ok := s.Offer()
	if !ok || files != 212 {
		t.Fatalf("files %d, ok %v", files, ok)
	}
	// 21 hours of sound is 5,040 windows at 0.28 s on the card, at the balanced speed: under an hour
	if seconds < 20*60 || seconds > 60*60 {
		t.Errorf("on the card it would take %d s", seconds)
	}
	s.OnCPU = func() bool { return true }
	_, onCPU, _ := s.Offer()
	if onCPU < 10*seconds {
		t.Errorf("the processor took %d s against %d s on the card", onCPU, seconds)
	}
}

func TestNoOfferUnlessEverythingIsInPlace(t *testing.T) {
	noSound := map[string]sidecar.RouteCount{"code": {Files: 9}}
	for name, change := range map[string]func(*Service, *fakeSettings, *fakeWaivers){
		"file search is off":              func(_ *Service, f *fakeSettings, _ *fakeWaivers) { f.cfg.Search.Enabled = false },
		"sound is already searched":       func(_ *Service, f *fakeSettings, _ *fakeWaivers) { f.cfg.Search.Audio = "content" },
		"the engine is not ready":         func(s *Service, _ *fakeSettings, _ *fakeWaivers) { s.Ready = func() bool { return false } },
		"not now was said recently":       func(_ *Service, _ *fakeSettings, w *fakeWaivers) { w.at = now.Add(-Quiet + time.Hour) },
		"the folders are not counted yet": func(s *Service, _ *fakeSettings, _ *fakeWaivers) { s.Surveys = fakeSurveys{testpath.P(`C:\Media`): {State: survey.Running}} },
		"there is no sound in the folders": func(s *Service, _ *fakeSettings, _ *fakeWaivers) {
			s.Surveys = fakeSurveys{testpath.P(`C:\Media`): {State: survey.Done, Survey: sidecar.Survey{Routes: noSound}}}
		},
	} {
		s, settings, waivers := world()
		change(s, settings, waivers)
		if _, _, ok := s.Offer(); ok {
			t.Errorf("%s, and the offer was made", name)
		}
	}
}

func TestNotNowLastsAMonthAndThenTheOfferReturns(t *testing.T) {
	s, _, waivers := world()
	if err := s.NotNow(); err != nil || !waivers.at.Equal(now) {
		t.Fatalf("NotNow: %v, remembered %v", err, waivers.at)
	}
	if _, _, ok := s.Offer(); ok {
		t.Fatal("offered again at once")
	}
	s.Now = func() time.Time { return now.Add(Quiet + time.Hour) }
	if _, _, ok := s.Offer(); !ok {
		t.Fatal("never offered again")
	}
}

func TestTurningItOnSetsTheChoice(t *testing.T) {
	s, settings, _ := world()
	if err := s.TurnOn(); err != nil || settings.cfg.Search.Audio != "content" {
		t.Fatalf("TurnOn: %v, audio = %q", err, settings.cfg.Search.Audio)
	}
	if _, _, ok := s.Offer(); ok {
		t.Fatal("still offered once it is on")
	}
}

func TestAFailureToRememberIsReported(t *testing.T) {
	s, _, waivers := world()
	waivers.err = errors.New("disk full")
	if err := s.NotNow(); err == nil {
		t.Fatal("a failure to save was swallowed")
	}
}
