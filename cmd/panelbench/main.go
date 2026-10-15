// Command panelbench measures the launcher's hotkey-to-search-bar latency against the real panel
// package, with another application holding the keyboard, the way a user meets it.
//
// It presses the hotkey itself 50 times, waits until the page reports that its input has focus
// and has painted, closes the panel, and checks that the keyboard went back to the other app.
// It exits with status 1 if the 95th percentile exceeds 100 ms or any run misbehaved
// (AGENTS.md: "the hotkey -> panel path must never wait on a model; opens in < 100 ms").
//
//	go run ./cmd/panelbench
//
// It briefly shows a console window and flashes the bar; do not type while it runs (about 40 s).
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"agi/internal/config"
	"agi/internal/panel"
)

const (
	runs        = 50
	limit       = 100 * time.Millisecond
	reportAfter = time.Second
)

const page = `<!doctype html><html><body style="margin:0;background:#000">
<input id="q" style="width:100%;padding:16px;background:#000;color:#fff;border:0;outline:0;font-size:20px">
<script>
const q = document.getElementById('q');
const send = ev => fetch('/m?ev=' + ev).catch(() => {});
window.addEventListener('focus', () => {
  q.focus(); q.select(); send('focus');
  requestAnimationFrame(() => requestAnimationFrame(() => send('painted')));
});
fetch('/ready');
</script></body></html>`

var (
	user32     = syscall.NewLazyDLL("user32.dll")
	keybdEvent = user32.NewProc("keybd_event")
	getFg      = user32.NewProc("GetForegroundWindow")
)

func foreground() uintptr {
	h, _, _ := getFg.Call()
	return h
}

func key(vk uintptr, up bool) {
	flags := uintptr(0)
	if up {
		flags = 2
	}
	_, _, _ = keybdEvent.Call(vk, 0, flags, 0)
}

// pressHotkey types Alt+Shift+Space, the default hotkey.
func pressHotkey() {
	for _, vk := range []uintptr{0x12, 0x10, 0x20} {
		key(vk, false)
	}
	for _, vk := range []uintptr{0x20, 0x10, 0x12} {
		key(vk, true)
	}
}

type events struct {
	mu   sync.Mutex
	seen map[string]time.Time
	wake chan struct{}
}

func (e *events) record(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, dup := e.seen[name]; !dup {
		e.seen[name] = time.Now()
		select {
		case e.wake <- struct{}{}:
		default:
		}
	}
}

func (e *events) reset() {
	e.mu.Lock()
	e.seen = map[string]time.Time{}
	e.mu.Unlock()
	select {
	case <-e.wake:
	default:
	}
}

func (e *events) get(name string) (time.Time, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	at, ok := e.seen[name]
	return at, ok
}

func main() {
	ev := &events{seen: map[string]time.Time{}, wake: make(chan struct{}, 1)}
	ready := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/ready", func(http.ResponseWriter, *http.Request) {
		select {
		case ready <- struct{}{}:
		default:
		}
	})
	mux.HandleFunc("/m", func(_ http.ResponseWriter, r *http.Request) { ev.record(r.URL.Query().Get("ev")) })
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, page)
	})

	app := application.New(application.Options{Name: "panelbench", Assets: application.AssetOptions{Handler: mux}})
	p := panel.New(app, config.DefaultHotkey)

	go func() { os.Exit(bench(p, ev, ready)) }()
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func bench(p *panel.Panel, ev *events, ready <-chan struct{}) int {
	<-ready
	time.Sleep(2 * time.Second) // the window prepares itself (cloak, first show, hotkey)
	if msg := p.HotkeyError(); msg != "" {
		fmt.Println("FAIL:", msg)
		return 1
	}

	other := exec.Command("cmd.exe", "/k", "title panelbench-other-app")
	other.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x10} // CREATE_NEW_CONSOLE
	if err := other.Start(); err != nil {
		fmt.Println("FAIL: cannot start the stand-in app:", err)
		return 1
	}
	defer func() { _ = other.Process.Kill() }()
	time.Sleep(2 * time.Second)
	otherWindow := foreground()

	var focus, painted []time.Duration
	problems := 0
	for i := 1; i <= runs; i++ {
		ev.reset()
		started := time.Now()
		pressHotkey()
		deadline := time.Now().Add(reportAfter)
		for time.Now().Before(deadline) {
			// The page reports focus and paint as two concurrent requests: wait for both.
			_, gotFocus := ev.get("focus")
			_, gotPaint := ev.get("painted")
			if gotFocus && gotPaint {
				break
			}
			select {
			case <-ev.wake:
			case <-time.After(5 * time.Millisecond):
			}
		}
		f, okF := ev.get("focus")
		pt, okP := ev.get("painted")
		switch {
		case !okF || !okP:
			fmt.Printf("run %2d: the page did not report focus and paint within %v (focus %v, painted %v; panel open %v, has the keyboard %v, bounds %+v)\n",
				i, reportAfter, okF, okP, p.Visible(), p.HasKeyboard(), p.Bounds())
			problems++
		case !p.Visible():
			fmt.Printf("run %2d: the panel is not open after the hotkey\n", i)
			problems++
		case !sane(p.Bounds()):
			fmt.Printf("run %2d: the panel has the wrong size or place: %+v\n", i, p.Bounds())
			problems++
		default:
			if i > 1 { // the first run warms the web view
				focus = append(focus, f.Sub(started))
				painted = append(painted, pt.Sub(started))
			}
		}
		p.Hide()
		time.Sleep(250 * time.Millisecond)
		if got := foreground(); got != otherWindow {
			fmt.Printf("run %2d: the keyboard did not return to the other app (foreground %#x, expected %#x)\n", i, got, otherWindow)
			problems++
		}
	}

	report := func(name string, d []time.Duration) time.Duration {
		if len(d) == 0 {
			fmt.Printf("%-18s no measurements\n", name)
			return 0
		}
		sort.Slice(d, func(a, b int) bool { return d[a] < d[b] })
		p50, p95, worst := d[len(d)/2], d[len(d)*95/100], d[len(d)-1]
		fmt.Printf("%-18s p50 %5.1f ms   p95 %5.1f ms   max %5.1f ms   (n=%d)\n", name, ms(p50), ms(p95), ms(worst), len(d))
		return p95
	}
	report("input focused", focus)
	p95 := report("painted", painted)
	fmt.Printf("problems: %d\n", problems)
	if problems > 0 || p95 > limit || len(painted) == 0 {
		fmt.Printf("FAIL: the 95th percentile must be under %v and every run must behave\n", limit)
		return 1
	}
	fmt.Println("PASS")
	return 0
}

// sane checks that the bar is at least 600 px wide, 60 px tall and on a screen (a failed
// monitor lookup once produced the minimum window size at the origin).
func sane(r panel.Rect) bool {
	return r.W >= 600 && r.H >= 60 && r.X+r.W > 0 && r.Y+r.H > 0 && r.Y < 4000
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
