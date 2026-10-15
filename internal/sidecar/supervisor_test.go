package sidecar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type exitErr struct{ code int }

func (e exitErr) Error() string { return fmt.Sprintf("exit status %d", e.code) }
func (e exitErr) ExitCode() int { return e.code }

// script says how one start of the fake engine behaves.
type script struct {
	output      string        // printed as soon as it starts
	notReady    int           // /health says "still loading" this many times first
	modelError  string        // /health reports this load failure instead
	exitAfter   time.Duration // the process ends by itself after this long (0 = never)
	exitCode    int
	startErr    error // Start itself fails
	silentAfter int   // after this many answers the port closes while the process lives on (0 = never)
}

// fakeEngine plays the Python process: a real HTTP server on the port the supervisor chose.
type fakeEngine struct {
	mu      sync.Mutex
	scripts []script // used in order; the last one repeats
	specs   []Spec
	kills   int
}

func (f *fakeEngine) next() script {
	f.mu.Lock()
	defer f.mu.Unlock()
	index := min(len(f.specs)-1, len(f.scripts)-1)
	return f.scripts[index]
}

func (f *fakeEngine) starts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.specs)
}

func (f *fakeEngine) killed() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.kills
}

func envValue(env []string, key string) string {
	for _, pair := range env {
		if value, ok := strings.CutPrefix(pair, key+"="); ok {
			return value
		}
	}
	return ""
}

func (f *fakeEngine) start(_ context.Context, spec Spec, output io.Writer) (Process, error) {
	f.mu.Lock()
	f.specs = append(f.specs, spec)
	f.mu.Unlock()
	sc := f.next()
	if sc.startErr != nil {
		return nil, sc.startErr
	}
	_, _ = io.WriteString(output, sc.output)

	token := envValue(spec.Env, "AGI_SIDECAR_TOKEN")
	listener, err := net.Listen("tcp", "127.0.0.1:"+envValue(spec.Env, "AGI_SIDECAR_PORT"))
	if err != nil {
		return nil, err
	}
	var answered int
	var answeredMu sync.Mutex
	var server *http.Server
	server = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token || r.URL.Path != "/health" {
			http.Error(w, `{"detail":"no"}`, http.StatusUnauthorized)
			return
		}
		answeredMu.Lock()
		answered++
		n := answered
		answeredMu.Unlock()
		health := Health{Ready: n > sc.notReady, Model: "fake", Version: "test"}
		if sc.modelError != "" {
			health.Ready, health.Error = false, &sc.modelError
		}
		_ = json.NewEncoder(w).Encode(health)
		if sc.silentAfter > 0 && n >= sc.silentAfter {
			go func() { _ = server.Close() }() // the port goes dead, the process lives on
		}
	})}
	go func() { _ = server.Serve(listener) }()

	p := &fakeProc{exit: make(chan error, 1), shutdown: func() { _ = server.Close() }}
	p.onKill = func() {
		f.mu.Lock()
		f.kills++
		f.mu.Unlock()
	}
	if sc.exitAfter > 0 {
		time.AfterFunc(sc.exitAfter, func() { p.finish(exitErr{sc.exitCode}) })
	}
	return p, nil
}

type fakeProc struct {
	exit     chan error
	once     sync.Once
	shutdown func()
	onKill   func()
}

func (p *fakeProc) finish(err error) {
	p.once.Do(func() {
		p.shutdown()
		p.exit <- err
	})
}

func (p *fakeProc) Wait() error { return <-p.exit }

func (p *fakeProc) Kill() error {
	p.onKill()
	p.finish(exitErr{1})
	return nil
}

// harness runs a supervisor against a fakeEngine and records every status it reports.
type harness struct {
	t        *testing.T
	engine   *fakeEngine
	sup      *Supervisor
	cancel   context.CancelFunc
	finished chan struct{}
	mu       sync.Mutex
	seen     []Status
}

func startHarness(t *testing.T, tune func(*Options), scripts ...script) *harness {
	t.Helper()
	h := &harness{t: t, engine: &fakeEngine{scripts: scripts}, finished: make(chan struct{})}
	opts := Options{
		Dir:            filepath.Join("C:", "repo", "sidecar"),
		Start:          h.engine.start,
		Backoff:        func(int) time.Duration { return 2 * time.Millisecond },
		Poll:           5 * time.Millisecond,
		StartupTimeout: 5 * time.Second,
		OnChange: func(s Status) {
			h.mu.Lock()
			h.seen = append(h.seen, s)
			h.mu.Unlock()
		},
	}
	if tune != nil {
		tune(&opts)
	}
	sup, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	h.sup = sup
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go func() {
		sup.Run(ctx)
		close(h.finished)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-h.finished:
		case <-time.After(5 * time.Second):
			t.Error("the supervisor did not stop")
		}
	})
	return h
}

func (h *harness) states() []State {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]State, len(h.seen))
	for i, s := range h.seen {
		out[i] = s.State
	}
	return out
}

func (h *harness) waitForState(want State) Status {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s := h.sup.Status(); s.State == want {
			return s
		}
		time.Sleep(2 * time.Millisecond)
	}
	h.t.Fatalf("never reached %q; states seen: %v, now %+v", want, h.states(), h.sup.Status())
	return Status{}
}

func (h *harness) waitUntilFinished() {
	h.t.Helper()
	select {
	case <-h.finished:
	case <-time.After(5 * time.Second):
		h.t.Fatalf("the supervisor kept running; states seen: %v", h.states())
	}
}

func contains(states []State, want ...State) bool {
	i := 0
	for _, s := range states {
		if i < len(want) && s == want[i] {
			i++
		}
	}
	return i == len(want)
}

func TestTheEngineIsStartedWithItsSettingsAndBecomesReady(t *testing.T) {
	h := startHarness(t, nil, script{notReady: 3})
	h.waitForState(StateReady)

	health, err := h.sup.Client().Health(context.Background())
	if err != nil || !health.Ready {
		t.Fatalf("health = %+v, err = %v", health, err)
	}
	if got := h.states(); !contains(got, StateStarting, StateWarming, StateReady) {
		t.Fatalf("states = %v, want starting, warming, ready in order", got)
	}

	spec := h.engine.specs[0]
	if want := filepath.Join("C:", "repo", "sidecar", ".venv", "Scripts", "python.exe"); spec.Exe != want {
		t.Errorf("program = %q, want the venv's python %q", spec.Exe, want)
	}
	if strings.Join(spec.Args, " ") != "-m agi_memory" || spec.Dir != filepath.Join("C:", "repo", "sidecar") {
		t.Errorf("args = %v, dir = %q", spec.Args, spec.Dir)
	}
	if token := envValue(spec.Env, "AGI_SIDECAR_TOKEN"); len(token) != 64 {
		t.Errorf("token = %q, want 64 hex characters", token)
	}
	if envValue(spec.Env, "AGI_SIDECAR_PORT") == "" || envValue(spec.Env, "HF_HUB_OFFLINE") != "1" {
		t.Errorf("environment is missing the port or HF_HUB_OFFLINE: %v", spec.Env)
	}
}

func TestEachSupervisorGetsItsOwnTokenAndPort(t *testing.T) {
	a, err := New(Options{Dir: "x"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(Options{Dir: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if a.token == b.token {
		t.Fatal("two supervisors share a token")
	}
	if a.port <= 0 || b.port <= 0 {
		t.Fatalf("ports = %d, %d", a.port, b.port)
	}
}

func TestStoppingKillsTheEngineAndEndsTheLoop(t *testing.T) {
	h := startHarness(t, nil, script{})
	h.waitForState(StateReady)
	h.cancel()
	h.waitUntilFinished()
	if h.sup.Status().State != StateStopped || h.engine.killed() != 1 {
		t.Fatalf("status = %+v, kills = %d", h.sup.Status(), h.engine.killed())
	}
	if _, err := h.sup.Client().Health(context.Background()); err == nil {
		t.Fatal("the engine still answers after stop")
	}
}

func TestAnEngineThatDiesIsStartedAgain(t *testing.T) {
	h := startHarness(t, nil,
		script{exitAfter: 80 * time.Millisecond, exitCode: 1, output: "Traceback\nRuntimeError: boom\n"},
		script{},
	)
	h.waitForState(StateReady)
	deadline := time.Now().Add(5 * time.Second)
	for h.engine.starts() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	status := h.waitForState(StateReady)
	if status.Restarts != 1 {
		t.Fatalf("restarts = %d, want 1", status.Restarts)
	}
	if got := h.states(); !contains(got, StateReady, StateRestarting, StateStarting, StateReady) {
		t.Fatalf("states = %v", got)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range h.seen {
		if s.State == StateRestarting && !strings.Contains(s.Message, "RuntimeError: boom") {
			t.Fatalf("the restart message %q does not say what the engine printed", s.Message)
		}
	}
}

func TestBadSettingsAreNotRetried(t *testing.T) {
	h := startHarness(t, nil, script{output: "agi_memory: AGI_SIDECAR_PORT must be a port number\n", exitAfter: 5 * time.Millisecond, exitCode: 2})
	h.waitUntilFinished()
	status := h.sup.Status()
	if status.State != StateFailed || !strings.Contains(status.Message, "AGI_SIDECAR_PORT must be a port number") {
		t.Fatalf("status = %+v", status)
	}
	if h.engine.starts() != 1 {
		t.Fatalf("started %d times, want 1", h.engine.starts())
	}
}

func TestAnIndexThatMustBeRebuiltIsNotRetriedAndSaysSo(t *testing.T) {
	h := startHarness(t, nil, script{
		output:    "agi_memory.store.flat.IndexMismatchError: index was built with (model, dim, schema) = ...: reindex required\n",
		exitAfter: 5 * time.Millisecond, exitCode: 1,
	})
	h.waitUntilFinished()
	status := h.sup.Status()
	if status.State != StateFailed || !strings.Contains(status.Message, "rebuilt") {
		t.Fatalf("status = %+v", status)
	}
	if h.engine.starts() != 1 {
		t.Fatalf("started %d times, want 1", h.engine.starts())
	}
}

func TestAMissingPythonIsFatalAndPointsAtUvSync(t *testing.T) {
	h := startHarness(t, nil, script{startErr: &fs.PathError{Op: "CreateProcess", Path: "python.exe", Err: fs.ErrNotExist}})
	h.waitUntilFinished()
	status := h.sup.Status()
	if status.State != StateFailed || !strings.Contains(status.Message, "uv sync") {
		t.Fatalf("status = %+v", status)
	}
	if other := (script{startErr: errors.New("access denied")}); other.startErr == nil {
		t.Fatal("unreachable")
	}
}

func TestAModelThatKeepsFailingIsGivenUpOn(t *testing.T) {
	h := startHarness(t, nil, script{modelError: "RuntimeError: no gpu driver"})
	h.waitUntilFinished()
	status := h.sup.Status()
	if status.State != StateFailed || !strings.Contains(status.Message, "no gpu driver") {
		t.Fatalf("status = %+v", status)
	}
	if h.engine.starts() != maxModelFailures {
		t.Fatalf("started %d times, want %d", h.engine.starts(), maxModelFailures)
	}
}

func TestAModelThatFailsOnceThenLoadsRecovers(t *testing.T) {
	h := startHarness(t, nil, script{modelError: "CUDA out of memory"}, script{})
	status := h.waitForState(StateReady)
	if status.Restarts != 1 || h.engine.starts() != 2 {
		t.Fatalf("restarts = %d, starts = %d", status.Restarts, h.engine.starts())
	}
}

func TestAnEngineThatStopsAnsweringIsRestarted(t *testing.T) {
	h := startHarness(t, func(o *Options) { o.Poll = 5 * time.Millisecond }, script{silentAfter: 4}, script{})
	deadline := time.Now().Add(5 * time.Second)
	for h.engine.starts() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if h.engine.starts() < 2 {
		t.Fatalf("the hung engine was never restarted; states: %v", h.states())
	}
	h.waitForState(StateReady)
	if h.engine.killed() < 1 {
		t.Fatal("the hung process was not killed")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	var said bool
	for _, s := range h.seen {
		said = said || strings.Contains(s.Message, "stopped answering")
	}
	if !said {
		t.Fatalf("no status explained the restart: %+v", h.seen)
	}
}

func TestAnEngineThatNeverFinishesLoadingIsRestarted(t *testing.T) {
	h := startHarness(t, func(o *Options) { o.StartupTimeout = 80 * time.Millisecond },
		script{notReady: 1 << 30}, script{})
	status := h.waitForState(StateReady)
	if status.Restarts < 1 {
		t.Fatalf("restarts = %d", status.Restarts)
	}
}

func TestBackoffGrowsAndIsCapped(t *testing.T) {
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second}
	for attempt, expected := range want {
		if got := defaultBackoff(attempt); got != expected {
			t.Errorf("defaultBackoff(%d) = %v, want %v", attempt, got, expected)
		}
	}
}

func TestExitCodeOfPlainErrorsIsUnknown(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{{nil, 0}, {exitErr{2}, 2}, {errors.New("boom"), -1}}
	for _, tt := range tests {
		if got := exitCode(tt.err); got != tt.want {
			t.Errorf("exitCode(%v) = %d, want %d", tt.err, got, tt.want)
		}
	}
}

func TestTailKeepsTheLastLinesAndTheLineInProgress(t *testing.T) {
	tl := newTail(3)
	_, _ = io.WriteString(tl, "one\r\ntwo\nthree\nfour\npart")
	if got := strings.Join(tl.Lines(), "|"); got != "two|three|four|part" {
		t.Fatalf("lines = %q", got)
	}
	_, _ = io.WriteString(tl, "ial\n\n\nfive\n")
	if got := tl.String(); got != "four\npartial\nfive" {
		t.Fatalf("tail = %q", got)
	}
}
