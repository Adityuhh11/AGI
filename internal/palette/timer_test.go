package palette

import (
	"strings"
	"testing"
	"time"

	"agi/internal/config"
	"agi/internal/timer"
)

type fakeTimer struct {
	state timer.State
	calls []string
}

func (f *fakeTimer) Start(d time.Duration, label string) {
	f.calls = append(f.calls, "start "+d.String()+" "+label)
}
func (f *fakeTimer) StartPomodoro()         { f.calls = append(f.calls, "pomodoro") }
func (f *fakeTimer) Pause()                 { f.calls = append(f.calls, "pause") }
func (f *fakeTimer) Resume()                { f.calls = append(f.calls, "resume") }
func (f *fakeTimer) Stop()                  { f.calls = append(f.calls, "stop") }
func (f *fakeTimer) Skip()                  { f.calls = append(f.calls, "skip") }
func (f *fakeTimer) State() timer.State     { return f.state }
func (f *fakeTimer) Settings() config.Timer { return config.Default().Timer }

func timerService(t *testing.T) (*Service, *fakeTimer, *int) {
	t.Helper()
	s, dismissed := newService(t, "Focus Keeper")
	ft := &fakeTimer{}
	With(s, Extras{Timer: ft})
	return s, ft, dismissed
}

// ids lists the timer rows of an answer (the web search row is always there as well).
func ids(a Answer) []string {
	var out []string
	for _, item := range a.Items {
		if item.Kind == "timer" {
			out = append(out, item.ID)
		}
	}
	return out
}

func TestTimerWithATimeOffersToStartIt(t *testing.T) {
	s, _, _ := timerService(t)
	tests := []struct{ text, id, title string }{
		{"timer 25", "timer:start:1500:", "Start a 25-minute timer"},
		{"timer 90s", "timer:start:90:", "Start a 90-second timer"},
		{"timer 1h30", "timer:start:5400:", "Start a 1 h 30 min timer"},
		{"timer 2h", "timer:start:7200:", "Start a 2-hour timer"},
		{"Timer 10m tea", "timer:start:600:tea", "Start a 10-minute timer: tea"},
		{"countdown 5", "timer:start:300:", "Start a 5-minute timer"},
		{"timer 2m30", "timer:start:150:", "Start a 2 min 30 s timer"},
	}
	for _, tt := range tests {
		items := s.Query(tt.text, ModeAll).Items
		if len(items) == 0 || items[0].ID != tt.id || items[0].Title != tt.title {
			t.Errorf("%q gave %+v, want first row %s %q", tt.text, items, tt.id, tt.title)
		}
	}
}

func TestPomodoroWordsOfferAPomodoroWithTheConfiguredLengths(t *testing.T) {
	s, _, _ := timerService(t)
	for _, text := range []string{"pomodoro", "pomo", "focus"} {
		items := s.Query(text, ModeAll).Items
		if len(items) == 0 || items[0].ID != "timer:pomodoro" || !strings.Contains(items[0].Title, "25 min focus, 5 min break") {
			t.Errorf("%q gave %+v", text, items)
		}
	}
}

func TestPlainTimerOffersPresetsWhenNothingRunsAndControlsWhenSomethingDoes(t *testing.T) {
	s, ft, _ := timerService(t)
	got := strings.Join(ids(s.Query("timer", ModeAll)), " ")
	if got != "timer:pomodoro timer:start:300: timer:start:600: timer:start:1500: timer:start:2700:" {
		t.Errorf("idle rows = %s", got)
	}
	ft.state = timer.State{Active: true, Running: true, Phase: timer.Focus, Next: "Short break"}
	if got := strings.Join(ids(s.Query("timer", ModeAll)), " "); got != "timer:pause timer:stop timer:skip" {
		t.Errorf("running rows = %s", got)
	}
	ft.state = timer.State{Active: true, Paused: true, Phase: timer.Countdown}
	if got := strings.Join(ids(s.Query("timer", ModeAll)), " "); got != "timer:resume timer:stop" {
		t.Errorf("paused rows = %s", got)
	}
	ft.state = timer.State{Active: true, Finished: true, Phase: timer.Focus, Next: "Short break"}
	if got := strings.Join(ids(s.Query("timer", ModeAll)), " "); got != "timer:stop timer:skip" {
		t.Errorf("finished rows = %s", got)
	}
}

func TestOrdinaryWordsAndOtherModesGetNoTimerRows(t *testing.T) {
	s, _, _ := timerService(t)
	for _, text := range []string{"time", "tim", "timers", "my timer 5", "timer abc", "timer 0", "timer 99h", "pomodoro 5"} {
		for _, id := range ids(s.Query(text, ModeAll)) {
			if strings.HasPrefix(id, "timer:") {
				t.Errorf("%q offered %s", text, id)
			}
		}
	}
	for _, id := range ids(s.Query("timer 5", ModeApps)) {
		if strings.HasPrefix(id, "timer:") {
			t.Errorf("the Apps mode offered %s", id)
		}
	}
}

func TestChoosingATimerRowDoesItAndClosesTheBar(t *testing.T) {
	s, ft, dismissed := timerService(t)
	for _, id := range []string{"timer:start:600:tea", "timer:pomodoro", "timer:pause", "timer:resume", "timer:skip", "timer:stop"} {
		if err := s.Do(id, "open", ""); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	want := "start 10m0s tea|pomodoro|pause|resume|skip|stop"
	if got := strings.Join(ft.calls, "|"); got != want || *dismissed != 6 {
		t.Errorf("calls %q (dismissed %d), want %q", got, *dismissed, want)
	}
	for _, bad := range []string{"timer:start:abc:", "timer:start:-5:", "timer:what"} {
		if err := s.Do(bad, "open", ""); err == nil {
			t.Errorf("%s was run", bad)
		}
	}
	if err := s.Do("timer:stop", "reveal", ""); err == nil {
		t.Error("a timer row can only be opened")
	}
}

func TestWithoutATimerTheBarSaysSoWhenAskedToRunOne(t *testing.T) {
	s, _ := newService(t)
	if got := ids(s.Query("timer 5", ModeAll)); len(got) != 0 {
		t.Errorf("rows without a timer: %v", got)
	}
	if err := s.Do("timer:stop", "open", ""); err == nil {
		t.Error("no timer, no error")
	}
}
