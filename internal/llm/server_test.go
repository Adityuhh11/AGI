package llm

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"omni/internal/sidecar"
)

func TestChooseDevicePicksTheDiscreteCardByNameNotByMemory(t *testing.T) {
	for name, tc := range map[string]struct{ listing, want string }{
		"intel and nvidia": {"Available devices:\n  Vulkan0: Intel(R) UHD Graphics (8038 MiB, 7380 MiB free)\n  Vulkan1: NVIDIA GeForce RTX 4050 Laptop GPU (5920 MiB, 5152 MiB free)\n", "Vulkan1"},
		"crlf":             {"  Vulkan0: Intel(R) UHD Graphics (8038 MiB, 7380 MiB free)\r\n  Vulkan1: NVIDIA GeForce RTX 3060 (12000 MiB, 9000 MiB free)\r\n", "Vulkan1"},
		"amd over unknown": {"  Vulkan0: Mystery GPU (2000 MiB, 1000 MiB free)\n  Vulkan1: AMD Radeon RX 6600 (8176 MiB, 8000 MiB free)\n", "Vulkan1"},
		"only integrated":  {"  Vulkan0: Intel(R) Iris(R) Xe Graphics (16000 MiB, 12000 MiB free)\n", ""},
		"unknown card":     {"  Vulkan0: Some New GPU (4000 MiB, 3000 MiB free)\n", "Vulkan0"},
		"no devices":       {"Available devices:\n", ""},
		"empty":            {"", ""},
	} {
		if got := ChooseDevice(tc.listing); got != tc.want {
			t.Errorf("%s: ChooseDevice = %q, want %q", name, got, tc.want)
		}
	}
}

// fakeTimers collects the idle timers the server sets, so a test fires them by hand.
type fakeTimers struct {
	mu      sync.Mutex
	pending []*fakeTimer
}

type fakeTimer struct {
	f       func()
	d       time.Duration
	stopped bool
}

func (t *fakeTimer) Stop() bool { was := !t.stopped; t.stopped = true; return was }

func (f *fakeTimers) after(d time.Duration, fn func()) Timer {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := &fakeTimer{f: fn, d: d}
	f.pending = append(f.pending, t)
	return t
}

// fire runs the newest timer that has not been stopped, and reports whether there was one.
func (f *fakeTimers) fire() bool {
	f.mu.Lock()
	var due *fakeTimer
	for _, t := range slices.Backward(f.pending) {
		if !t.stopped {
			due = t
			break
		}
	}
	f.mu.Unlock()
	if due == nil {
		return false
	}
	due.f()
	return true
}

// standIn plays llama-server: it listens on the port it is given and answers /health, unless it is told to fail.
type standIn struct {
	mu       sync.Mutex
	starts   [][]string // the arguments of every start
	failGPU  bool       // a start with a --device other than none exits at once
	slow     time.Duration
	procs    []*fakeProc
	startErr error
}

type fakeProc struct {
	ln   net.Listener
	done chan struct{}
	once sync.Once
	err  error
}

func (p *fakeProc) Wait() error { <-p.done; return p.err }
func (p *fakeProc) Kill() error { p.end(nil); return nil }
func (p *fakeProc) end(err error) {
	p.once.Do(func() {
		p.err = err
		if p.ln != nil {
			_ = p.ln.Close()
		}
		close(p.done)
	})
}

func (s *standIn) start(_ context.Context, spec sidecar.Spec, output io.Writer) (sidecar.Process, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.startErr != nil {
		return nil, s.startErr
	}
	s.starts = append(s.starts, spec.Args)
	proc := &fakeProc{done: make(chan struct{})}
	s.procs = append(s.procs, proc)
	device := ""
	for i, a := range spec.Args {
		if a == "--device" {
			device = spec.Args[i+1]
		}
	}
	if s.failGPU && device != "none" {
		_, _ = io.WriteString(output, "ggml_vulkan: out of device memory\n")
		proc.end(errors.New("exit status 1"))
		return proc, nil
	}
	port := ""
	for i, a := range spec.Args {
		if a == "--port" {
			port = spec.Args[i+1]
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		return nil, err
	}
	proc.ln = ln
	slow := s.slow
	go func() {
		_ = http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			time.Sleep(slow)
			w.WriteHeader(http.StatusOK)
		}))
	}()
	return proc, nil
}

func (s *standIn) count() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.starts) }

func rig(t *testing.T, idleMinutes int) (*Server, *standIn, *fakeTimers, *[]Status) {
	t.Helper()
	root := t.TempDir()
	layout := Layout{Root: root}
	for _, p := range []string{layout.Server(), layout.ModelFile(), layout.Manifest()} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fake, timers := &standIn{}, &fakeTimers{}
	var mu sync.Mutex
	var seen []Status
	srv := NewServer(Options{
		Layout:        func() Layout { return layout },
		ContextTokens: func() int { return 8192 },
		IdleMinutes:   func() int { return idleMinutes },
		Start:         fake.start,
		Devices: func(context.Context, string) (string, error) {
			return "  Vulkan1: NVIDIA GeForce RTX 4050 Laptop GPU (5920 MiB, 5152 MiB free)\n", nil
		},
		After:        timers.after,
		StartTimeout: 5 * time.Second,
		OnChange:     func(st Status) { mu.Lock(); seen = append(seen, st); mu.Unlock() },
	})
	t.Cleanup(srv.Unload)
	return srv, fake, timers, &seen
}

func TestEnsureStartsTheServerOnTheGraphicsCardOnceForManyAsks(t *testing.T) {
	srv, fake, _, _ := rig(t, 10)
	fake.slow = 200 * time.Millisecond
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := srv.Ensure(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if fake.count() != 1 {
		t.Fatalf("the server was started %d times for 4 askers, want 1", fake.count())
	}
	args := fake.starts[0]
	for _, want := range []string{"--jinja", "--no-webui", "-np", "1", "-c", "8192", "-ngl", "99", "Vulkan1"} {
		if !slices.Contains(args, want) {
			t.Errorf("arguments %v lack %q", args, want)
		}
	}
	if st := srv.Status(); st.State != Ready || st.Device != "gpu" || st.UnloadsAt.IsZero() {
		t.Fatalf("status = %+v, want ready on the gpu with an unload time", st)
	}
	ep, _ := srv.Ensure(context.Background())
	if ep.Key == "" || ep.URL == "" || fake.count() != 1 {
		t.Fatalf("a ready server must be returned as it is: %+v", ep)
	}
}

func TestAGraphicsCardThatFailsFallsBackToTheProcessor(t *testing.T) {
	srv, fake, _, _ := rig(t, 10)
	fake.failGPU = true
	ep, err := srv.Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ep.Device != "cpu" || fake.count() != 2 || !slices.Contains(fake.starts[1], "none") {
		t.Fatalf("device %q after %d starts (%v); want the processor on the second", ep.Device, fake.count(), fake.starts)
	}
}

func TestUseProcessorKeepsTheModelOffTheCardAfterwards(t *testing.T) {
	srv, fake, _, _ := rig(t, 10)
	if _, err := srv.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv.UseProcessor()
	ep, err := srv.Ensure(context.Background())
	if err != nil || ep.Device != "cpu" || !slices.Contains(fake.starts[len(fake.starts)-1], "none") {
		t.Fatalf("after UseProcessor: device %q, err %v", ep.Device, err)
	}
}

func TestAServerThatCannotStartFailsWithWhatItPrintedAndCanBeTriedAgain(t *testing.T) {
	srv, fake, _, _ := rig(t, 10)
	fake.startErr = errors.New("no such file")
	if _, err := srv.Ensure(context.Background()); err == nil {
		t.Fatal("a server that cannot be started was reported ready")
	}
	if st := srv.Status(); st.State != Failed || st.Message == "" {
		t.Fatalf("status = %+v, want failed with a reason", st)
	}
	fake.mu.Lock()
	fake.startErr = nil
	fake.mu.Unlock()
	if _, err := srv.Ensure(context.Background()); err != nil {
		t.Fatalf("the next ask should start it again: %v", err)
	}
}

func TestUnloadStopsTheServerAndTheNextAskStartsANewOne(t *testing.T) {
	srv, fake, _, _ := rig(t, 10)
	first, _ := srv.Ensure(context.Background())
	srv.Unload()
	if st := srv.Status(); st.State != Unloaded {
		t.Fatalf("status after Unload = %+v", st)
	}
	select {
	case <-fake.procs[0].done:
	case <-time.After(2 * time.Second):
		t.Fatal("the server process was not stopped")
	}
	second, err := srv.Ensure(context.Background())
	if err != nil || fake.count() != 2 || second.Key == first.Key {
		t.Fatalf("a new server with a new key was expected: err %v, starts %d", err, fake.count())
	}
}

func TestAnIdleModelIsUnloadedButNotWhileItIsHeld(t *testing.T) {
	srv, _, timers, _ := rig(t, 5)
	if _, err := srv.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if timers.pending[len(timers.pending)-1].d != 5*time.Minute {
		t.Fatalf("idle time = %v, want 5 minutes", timers.pending[len(timers.pending)-1].d)
	}
	release := srv.Hold()
	if timers.fire() {
		t.Fatal("an idle timer was still pending while the model was in use")
	}
	if srv.Status().State != Ready {
		t.Fatal("the model was unloaded while held")
	}
	release()
	release() // releasing twice must not unbalance the count
	if !timers.fire() {
		t.Fatal("releasing the hold should start the idle clock")
	}
	if srv.Status().State != Unloaded {
		t.Fatalf("status after the idle time = %+v, want unloaded", srv.Status())
	}
}

func TestZeroMinutesMeansTheModelIsNeverUnloaded(t *testing.T) {
	srv, _, timers, _ := rig(t, 0)
	if _, err := srv.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if timers.fire() || !srv.Status().UnloadsAt.IsZero() {
		t.Fatal("a model set to stay loaded got an idle timer")
	}
}

func TestAServerThatDiesIsStartedAgainByTheNextAsk(t *testing.T) {
	srv, fake, _, _ := rig(t, 10)
	if _, err := srv.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.procs[0].end(errors.New("killed"))
	deadline := time.Now().Add(2 * time.Second)
	for srv.Status().State == Ready && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if st := srv.Status(); st.State != Unloaded || st.Message == "" {
		t.Fatalf("status after the server died = %+v", st)
	}
	if _, err := srv.Ensure(context.Background()); err != nil || fake.count() != 2 {
		t.Fatalf("err %v, starts %d; want a second start", err, fake.count())
	}
}

func TestAskingWithoutAnInstallSaysSo(t *testing.T) {
	srv := NewServer(Options{
		Layout:        func() Layout { return Layout{Root: t.TempDir()} },
		ContextTokens: func() int { return 8192 },
		IdleMinutes:   func() int { return 10 },
	})
	if _, err := srv.Ensure(context.Background()); err == nil || srv.Status().State != Failed {
		t.Fatalf("err %v, status %+v; want a failure that says it is not installed", err, srv.Status())
	}
}

func TestACancelledAskDoesNotStopTheStartForOthers(t *testing.T) {
	srv, fake, _, _ := rig(t, 10)
	fake.slow = 400 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := srv.Ensure(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the caller's deadline", err)
	}
	if _, err := srv.Ensure(context.Background()); err != nil || fake.count() != 1 {
		t.Fatalf("err %v, starts %d; the warm-up should have carried on", err, fake.count())
	}
}

func TestEveryChangeIsPublished(t *testing.T) {
	srv, _, _, seen := rig(t, 10)
	if _, err := srv.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv.Unload()
	var states []State
	for _, st := range *seen {
		if len(states) == 0 || states[len(states)-1] != st.State {
			states = append(states, st.State)
		}
	}
	if !slices.Equal(states, []State{Loading, Ready, Unloaded}) {
		t.Fatalf("states published = %v", states)
	}
}
