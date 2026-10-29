package llm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"omni/internal/sidecar"
)

// State is where the model is: not in memory, being loaded, answering, or unable to start.
type State string

const (
	Unloaded State = "unloaded"
	Loading  State = "loading"
	Ready    State = "ready"
	Failed   State = "failed"
)

// Status is the model as the settings page and the chat window show it.
type Status struct {
	State   State
	Message string // what is happening, or why it failed
	// Device is where the model runs once loaded: "gpu" or "cpu".
	Device string
	// UnloadsAt is when an idle model will be unloaded; zero while it is busy or never unloads.
	UnloadsAt time.Time
}

// Endpoint is how to talk to the running server: an OpenAI-style API on a loopback port, behind a key only Omni knows (a web
// page in the person's browser could otherwise call a local server).
type Endpoint struct {
	URL, Key string
	Device   string
}

// Timer is what After returns: something that can be stopped.
type Timer interface{ Stop() bool }

// Options are what a Server needs; the zero values of the optional ones are right outside tests.
type Options struct {
	// Layout, ContextTokens and IdleMinutes are read each time they matter, so a changed setting applies without a restart.
	Layout        func() Layout
	ContextTokens func() int
	IdleMinutes   func() int
	// Start runs the program; Devices lists the graphics devices; After runs f later; Now is the clock. Tests replace them.
	Start   sidecar.Starter
	Devices func(ctx context.Context, exe string) (string, error)
	After   func(d time.Duration, f func()) Timer
	Now     func() time.Time
	// StartTimeout is how long one device gets to become healthy. OnChange hears every change of the status.
	StartTimeout time.Duration
	OnChange     func(Status)
	// Logf receives what went wrong, with the server's last output.
	Logf func(format string, args ...any)
}

// startTimeout is generous: a first start after the download took 11 s, and a start on the processor is slower.
const startTimeout = 60 * time.Second

// Server is the llama.cpp server for the model: started when it is needed, stopped when it has been idle.
type Server struct {
	o Options

	mu       sync.Mutex
	state    State
	message  string
	device   string
	cpuOnly  bool // the graphics card failed; stay on the processor until Omni restarts
	endpoint Endpoint
	proc     sidecar.Process
	loading  *load
	cancel   context.CancelFunc // cancels the load under way
	holds    int
	idle     Timer
	gen      int // changes whenever the idle timer is set or stopped, so a timer that already fired is ignored
	deadline time.Time
}

// load is one start of the server; everyone who asked for the model while it starts waits on done.
type load struct {
	done chan struct{}
	err  error
}

// NewServer returns a Server that has not started anything.
func NewServer(o Options) *Server {
	if o.Start == nil {
		o.Start = sidecar.ExecStarter
	}
	if o.Devices == nil {
		o.Devices = listDevices
	}
	if o.After == nil {
		o.After = func(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.StartTimeout == 0 {
		o.StartTimeout = startTimeout
	}
	if o.Logf == nil {
		o.Logf = func(string, ...any) {}
	}
	return &Server{o: o, state: Unloaded}
}

// Status reports the model now.
func (s *Server) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

func (s *Server) statusLocked() Status {
	return Status{State: s.state, Message: s.message, Device: s.device, UnloadsAt: s.deadline}
}

// set changes the state under the lock and returns the status to publish once the lock is released.
func (s *Server) set(state State, message string) Status {
	s.state, s.message = state, message
	if state != Ready {
		s.device = ""
	}
	return s.statusLocked()
}

func (s *Server) publish(st Status) {
	if s.o.OnChange != nil {
		s.o.OnChange(st)
	}
}

// Ensure makes sure the model is running and returns how to reach it. The start goes on in the background if the caller gives
// up, so a warm-up nobody waits for still finishes.
func (s *Server) Ensure(ctx context.Context) (Endpoint, error) {
	for range 3 { // the model can be unloaded again between its becoming ready and our looking
		s.mu.Lock()
		if s.state == Ready {
			ep := s.endpoint
			s.mu.Unlock()
			return ep, nil
		}
		l := s.loading
		var published *Status
		if l == nil {
			l = &load{done: make(chan struct{})}
			s.loading = l
			loadCtx, cancel := context.WithCancel(context.Background())
			s.cancel = cancel
			st := s.set(Loading, "Starting the model…")
			published = &st
			go s.run(loadCtx, l)
		}
		s.mu.Unlock()
		if published != nil {
			s.publish(*published)
		}
		select {
		case <-l.done:
		case <-ctx.Done():
			return Endpoint{}, ctx.Err()
		}
		if l.err != nil {
			return Endpoint{}, l.err
		}
	}
	return Endpoint{}, errors.New("the model was stopped while it started")
}

// run starts the server and records the outcome.
func (s *Server) run(ctx context.Context, l *load) {
	ep, proc, exited, err := s.start(ctx)
	s.mu.Lock()
	s.loading = nil
	s.cancel = nil
	var st Status
	switch {
	case err == nil && ctx.Err() != nil: // unloaded while it was starting
		err = ctx.Err()
		st = s.set(Unloaded, "")
	case err != nil:
		st = s.set(Failed, err.Error())
	default:
		s.endpoint, s.proc = ep, proc
		s.device = ep.Device
		st = s.set(Ready, "")
		s.armLocked()
		st = s.statusLocked()
		go s.watch(proc, exited)
	}
	l.err = err
	s.mu.Unlock()
	if err != nil && ctx.Err() != nil {
		killQuietly(proc)
	}
	s.publish(st)
	close(l.done)
}

// watch notices a server that ends without being asked to; the next question starts it again.
func (s *Server) watch(proc sidecar.Process, exited <-chan error) {
	<-exited
	s.mu.Lock()
	if s.proc != proc {
		s.mu.Unlock()
		return
	}
	s.proc = nil
	s.stopIdleLocked()
	st := s.set(Unloaded, "The model stopped by itself; it starts again with the next question.")
	s.mu.Unlock()
	s.o.Logf("chat: the model server ended unexpectedly")
	s.publish(st)
}

func killQuietly(proc sidecar.Process) {
	if proc != nil {
		_ = proc.Kill()
	}
}

// start picks the device and launches the server on it, falling back to the processor.
func (s *Server) start(ctx context.Context) (Endpoint, sidecar.Process, <-chan error, error) {
	layout := s.o.Layout()
	if !layout.Installed() {
		return Endpoint{}, nil, nil, errors.New("Omni AI is not installed")
	}
	s.mu.Lock()
	cpuOnly := s.cpuOnly
	s.mu.Unlock()
	attempts := []string{""}
	if !cpuOnly {
		listing, err := s.o.Devices(ctx, layout.Server())
		if err != nil {
			s.o.Logf("chat: %v; using the processor", err)
		}
		if id := ChooseDevice(listing); id != "" {
			attempts = []string{id, ""}
		}
	}
	var last error
	for _, id := range attempts {
		ep, proc, exited, err := s.launch(ctx, layout, id)
		if err == nil {
			return ep, proc, exited, nil
		}
		last = err
		if ctx.Err() != nil {
			break
		}
		if id != "" {
			s.o.Logf("chat: the model did not start on %s; trying the processor: %v", id, err)
		}
	}
	return Endpoint{}, nil, nil, last
}

// launch starts the server on one device ("" is the processor) and waits until it answers /health.
func (s *Server) launch(ctx context.Context, l Layout, deviceID string) (Endpoint, sidecar.Process, <-chan error, error) {
	port, err := freePort()
	if err != nil {
		return Endpoint{}, nil, nil, err
	}
	key, err := randomKey()
	if err != nil {
		return Endpoint{}, nil, nil, err
	}
	offload := []string{"-ngl", "0", "--device", "none"}
	device := "cpu"
	if deviceID != "" {
		offload, device = []string{"-ngl", "99", "--device", deviceID}, "gpu"
	}
	args := append([]string{
		"-m", l.ModelFile(),
		"--host", "127.0.0.1", "--port", strconv.Itoa(port), "--api-key", key,
		"--no-webui",
		"--jinja",
		"-c", strconv.Itoa(s.o.ContextTokens()),
		"-np", "1", // one slot: more would split the context between them
		"-lv", "2", "--log-colors", "off",
	}, offload...)
	tail := &tail{max: 20}
	proc, err := s.o.Start(ctx, sidecar.Spec{Dir: l.Dir(), Exe: l.Server(), Args: args}, tail)
	if err != nil {
		return Endpoint{}, nil, nil, fmt.Errorf("start the model server: %w", err)
	}
	exited := make(chan error, 1) // read here while the server starts, and by watch once it is up
	go func() { exited <- proc.Wait() }()

	ep := Endpoint{URL: "http://127.0.0.1:" + strconv.Itoa(port), Key: key, Device: device}
	deadline := time.NewTimer(s.o.StartTimeout)
	defer deadline.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-exited:
			return Endpoint{}, nil, nil, fmt.Errorf("the model server stopped while starting (%v): %s", err, tail)
		case <-ctx.Done():
			killQuietly(proc)
			return Endpoint{}, nil, nil, ctx.Err()
		case <-deadline.C:
			killQuietly(proc)
			return Endpoint{}, nil, nil, fmt.Errorf("the model server was not ready after %v: %s", s.o.StartTimeout, tail)
		case <-tick.C:
			if healthy(ctx, ep) {
				return ep, proc, exited, nil
			}
		}
	}
}

func healthy(ctx context.Context, ep Endpoint) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep.URL+"/health", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+ep.Key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("find a free port: %w", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func randomKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("make the server key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Hold marks the model as in use until the returned function is called, so it is never unloaded in the middle of an answer. The
// idle time counts from the last release.
func (s *Server) Hold() (release func()) {
	s.mu.Lock()
	s.holds++
	s.stopIdleLocked()
	st := s.statusLocked()
	s.mu.Unlock()
	s.publish(st)
	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			s.holds--
			s.armLocked()
			st := s.statusLocked()
			s.mu.Unlock()
			s.publish(st)
		})
	}
}

// Rearm applies a changed idle time to a model that is waiting to be unloaded.
func (s *Server) Rearm() {
	s.mu.Lock()
	s.armLocked()
	st := s.statusLocked()
	s.mu.Unlock()
	s.publish(st)
}

func (s *Server) stopIdleLocked() {
	s.gen++
	if s.idle != nil {
		s.idle.Stop()
		s.idle = nil
	}
	s.deadline = time.Time{}
}

// armLocked starts the idle clock if the model is loaded and nobody is using it.
func (s *Server) armLocked() {
	s.stopIdleLocked()
	minutes := s.o.IdleMinutes()
	if minutes <= 0 || s.state != Ready || s.holds > 0 {
		return
	}
	wait := time.Duration(minutes) * time.Minute
	gen := s.gen
	s.deadline = s.o.Now().Add(wait)
	s.idle = s.o.After(wait, func() {
		s.mu.Lock()
		stale := gen != s.gen || s.holds > 0 || s.state != Ready
		s.mu.Unlock()
		if !stale {
			s.Unload()
		}
	})
}

// Unload stops the server and so frees the memory it held, graphics memory included. A start under way is cancelled.
func (s *Server) Unload() {
	s.mu.Lock()
	s.stopIdleLocked()
	if s.cancel != nil {
		s.cancel()
	}
	proc := s.proc
	s.proc = nil
	changed := proc != nil || s.state == Failed
	var st Status
	if changed {
		st = s.set(Unloaded, "")
	}
	s.mu.Unlock()
	killQuietly(proc)
	if changed {
		s.publish(st)
	}
}

// UseProcessor keeps the model off the graphics card from now until Omni restarts: the card ran out of memory or stopped.
func (s *Server) UseProcessor() {
	s.mu.Lock()
	s.cpuOnly = true
	s.mu.Unlock()
	s.Unload()
}

// tail keeps the last lines the server printed, so a failure can say what it said.
type tail struct {
	mu      sync.Mutex
	partial string
	lines   []string
	max     int
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.partial += string(p)
	for {
		i := strings.IndexByte(t.partial, '\n')
		if i < 0 {
			break
		}
		if line := strings.TrimRight(t.partial[:i], "\r"); line != "" {
			t.lines = append(t.lines, line)
		}
		t.partial = t.partial[i+1:]
	}
	if extra := len(t.lines) - t.max; extra > 0 {
		t.lines = t.lines[extra:]
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.Join(t.lines, "\n")
}
