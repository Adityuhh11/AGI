package palette

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"agi/internal/config"
	"agi/internal/timer"
)

// TimerControl is the timer as the bar uses it; *timer.Timer satisfies it.
type TimerControl interface {
	Start(d time.Duration, label string)
	StartPomodoro()
	Pause()
	Resume()
	Stop()
	Skip()
	State() timer.State
	Settings() config.Timer
}

// timerPresets are the lengths offered when only "timer" is typed, in minutes.
var timerPresets = []int{5, 10, 25, 45}

// timerWords are the first words that ask for the timer. Whole words only: "time" is far too common a thing to search for.
var timerWords = map[string]bool{"timer": true, "countdown": true}

// pomodoroWords start a pomodoro.
var pomodoroWords = map[string]bool{"pomodoro": true, "pomo": true}

// timerItems are the rows the timer offers for the text: the controls of a running timer, a pomodoro, or a countdown.
func (s *Service) timerItems(text string) []Item {
	if s.x.Timer == nil {
		return nil
	}
	first, rest, _ := strings.Cut(strings.TrimSpace(text), " ")
	first, rest = strings.ToLower(first), strings.TrimSpace(rest)
	var items []Item
	switch {
	case pomodoroWords[first] && rest == "":
		items = append(items, s.pomodoroItem())
	case timerWords[first] && rest == "":
		items = append(items, s.timerControls()...)
		if !s.x.Timer.State().Active {
			items = append(items, s.pomodoroItem())
			for _, m := range timerPresets {
				items = append(items, countdownItem(time.Duration(m)*time.Minute, ""))
			}
		}
	case timerWords[first]:
		if d, label, ok := timer.ParseDuration(rest); ok {
			items = append(items, countdownItem(d, label))
		}
	}
	return items
}

func (s *Service) pomodoroItem() Item {
	c := s.x.Timer.Settings()
	return Item{
		ID: "timer:pomodoro", Kind: "timer",
		Title:    fmt.Sprintf("Start a pomodoro: %d min focus, %d min break", c.FocusMinutes, c.ShortBreakMinutes),
		Subtitle: "Timer",
	}
}

func countdownItem(d time.Duration, label string) Item {
	title := "Start a " + sayDuration(d) + " timer"
	if label != "" {
		title += ": " + label
	}
	return Item{ID: "timer:start:" + strconv.Itoa(int(d/time.Second)) + ":" + label, Kind: "timer", Title: title, Subtitle: "Timer"}
}

// timerControls are the rows that act on the timer that is running (none if there is not one).
func (s *Service) timerControls() []Item {
	st := s.x.Timer.State()
	if !st.Active {
		return nil
	}
	items := []Item{{ID: "timer:stop", Kind: "timer", Title: "Stop the timer", Subtitle: "Timer"}}
	if !st.Finished {
		if st.Paused {
			items = append([]Item{{ID: "timer:resume", Kind: "timer", Title: "Resume the timer", Subtitle: "Timer"}}, items...)
		} else {
			items = append([]Item{{ID: "timer:pause", Kind: "timer", Title: "Pause the timer", Subtitle: "Timer"}}, items...)
		}
	}
	if st.Next != "" {
		items = append(items, Item{ID: "timer:skip", Kind: "timer", Title: "Skip to the " + strings.ToLower(st.Next), Subtitle: "Timer"})
	}
	return items
}

// sayDuration is a length of time in words: "25-minute", "1 h 30 min", "90-second".
func sayDuration(d time.Duration) string {
	secs := int(d / time.Second)
	switch {
	case secs%3600 == 0:
		return strconv.Itoa(secs/3600) + "-hour"
	case secs >= 3600:
		return fmt.Sprintf("%d h %d min", secs/3600, secs%3600/60)
	case secs%60 == 0:
		return strconv.Itoa(secs/60) + "-minute"
	case secs < 60 || (secs < 120 && secs%60 != 0):
		return strconv.Itoa(secs) + "-second"
	}
	return fmt.Sprintf("%d min %d s", secs/60, secs%60)
}

// doTimer carries out a timer row and closes the bar.
func (s *Service) doTimer(rest, action string) error {
	if action != "open" {
		return fmt.Errorf("a timer row can only be opened, not %q", action)
	}
	if s.x.Timer == nil {
		return fmt.Errorf("the timer is not available")
	}
	var run func()
	switch {
	case rest == "pomodoro":
		run = s.x.Timer.StartPomodoro
	case rest == "pause":
		run = s.x.Timer.Pause
	case rest == "resume":
		run = s.x.Timer.Resume
	case rest == "stop":
		run = s.x.Timer.Stop
	case rest == "skip":
		run = s.x.Timer.Skip
	case strings.HasPrefix(rest, "start:"):
		seconds, label, _ := strings.Cut(strings.TrimPrefix(rest, "start:"), ":")
		n, err := strconv.Atoi(seconds)
		if err != nil || n <= 0 {
			return fmt.Errorf("%q is not a length of time", seconds)
		}
		run = func() { s.x.Timer.Start(time.Duration(n)*time.Second, label) }
	default:
		return fmt.Errorf("do not know how to run timer:%s", rest)
	}
	s.dismiss() // the bar goes first: starting a timer builds a window, and the bar should not wait for that
	run()
	return nil
}
