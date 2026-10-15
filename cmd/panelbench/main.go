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
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"agi"
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

// pillPage is the stand-in for the designed bar: a transparent page, a pill with a CSS shadow, and
// an animated WebGL ring behind it, the costly parts of the design (the animation pauses when the
// panel loses the keyboard, as the real page must).
const pillPage = `<!doctype html><html><body style="margin:0;background:transparent;overflow:hidden">
<div style="padding:22px">
<div id="pill" style="position:relative;height:60px;border-radius:30px;box-shadow:0 12px 40px rgba(0,0,0,.55),0 2px 8px rgba(0,0,0,.4)">
<canvas id="gl" style="position:absolute;inset:0;width:100%;height:100%;border-radius:30px"></canvas>
<div style="position:absolute;inset:3px;border-radius:27px;background:#0b0b0c">
<input id="q" style="width:100%;height:100%;box-sizing:border-box;padding:0 28px;background:transparent;color:#fff;border:0;outline:0;font-size:20px"></div>
</div></div>
<script>
const q = document.getElementById('q');
const send = ev => fetch('/m?ev=' + ev).catch(() => {});
const gl = document.getElementById('gl').getContext('webgl');
const c = gl.canvas, dpr = window.devicePixelRatio || 1;
c.width = 700 * dpr; c.height = 60 * dpr;
const sh = (t, src) => { const s = gl.createShader(t); gl.shaderSource(s, src); gl.compileShader(s); return s; };
const pr = gl.createProgram();
gl.attachShader(pr, sh(gl.VERTEX_SHADER, 'attribute vec2 p;void main(){gl_Position=vec4(p,0.,1.);}'));
gl.attachShader(pr, sh(gl.FRAGMENT_SHADER, 'precision mediump float;uniform float t;uniform vec2 r;void main(){vec2 u=gl_FragCoord.xy/r;float v=sin(u.x*6.+t*.8)+.5*sin(u.x*11.-t*1.3+u.y*4.)+.7*sin(u.x*3.+u.y*9.+t*.5);float m=.5+.5*sin(v*2.);gl_FragColor=vec4(mix(vec3(.35,.37,.42),vec3(.95,.97,1.),pow(m,2.5)),1.);}'));
gl.linkProgram(pr); gl.useProgram(pr);
const b = gl.createBuffer(); gl.bindBuffer(gl.ARRAY_BUFFER, b);
gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1,-1,1,-1,-1,1,1,1]), gl.STATIC_DRAW);
const loc = gl.getAttribLocation(pr, 'p'); gl.enableVertexAttribArray(loc); gl.vertexAttribPointer(loc, 2, gl.FLOAT, false, 0, 0);
gl.viewport(0, 0, c.width, c.height); gl.uniform2f(gl.getUniformLocation(pr, 'r'), c.width, c.height);
const tu = gl.getUniformLocation(pr, 't');
let running = true, last = 0;
function frame(now) { if (!running) return; if (now - last > 33) { last = now; gl.uniform1f(tu, now / 1000); gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4); } requestAnimationFrame(frame); }
requestAnimationFrame(frame);
window.addEventListener('blur', () => { running = false; });
window.addEventListener('focus', () => {
  if (!running) { running = true; requestAnimationFrame(frame); }
  q.focus(); q.select(); send('focus');
  requestAnimationFrame(() => requestAnimationFrame(() => send('painted')));
});
fetch('/ready');
</script></body></html>`

// probe is added to the real page so the bench can see when it has focus and has painted.
const probe = `<script>
const send = ev => fetch('/m?ev=' + ev).catch(() => {});
window.addEventListener('focus', () => {
  send('focus');
  requestAnimationFrame(() => requestAnimationFrame(() => send('painted')));
});
fetch('/ready');
</script>`

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

var (
	usePill = flag.Bool("pill", false, "measure the pill-with-shader stand-in page instead of the plain bar")
	hold    = flag.Duration("hold", 0, "keep the panel open this long after the first run, to look at it")
	real    = flag.Bool("real", false, "measure the launcher's real page (frontend/dist) with a probe script added")
)

func main() {
	flag.Parse()
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
	if *real {
		dist, err := fs.Sub(agi.Frontend, "frontend/dist")
		if err != nil {
			log.Fatal(err)
		}
		files := http.FileServerFS(dist)
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" && r.URL.Path != "/index.html" {
				files.ServeHTTP(w, r)
				return
			}
			index, err := fs.ReadFile(dist, "index.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, strings.Replace(string(index), "</body>", probe+"</body>", 1))
		})
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			if *usePill {
				_, _ = fmt.Fprint(w, pillPage)
				return
			}
			_, _ = fmt.Fprint(w, page)
		})
	}

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
		if i == 1 && *hold > 0 {
			fmt.Printf("HOLD %+v\n", p.Bounds())
			time.Sleep(*hold)
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
