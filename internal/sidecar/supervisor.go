package sidecar

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// State is where the engine process is in its life.
type State string

const (
	// StateStarting: the process is being started or has just started; nothing listens yet.
	StateStarting State = "starting"
	// StateWarming: the process answers /health but the model is still loading (10-35 s).
	StateWarming State = "warming"
	// StateReady: search and indexing work.
	StateReady State = "ready"
	// StateRestarting: the process died or hung; it starts again after a pause.
	StateRestarting State = "restarting"
	// StateFailed: it will not be restarted; Message says why and what to do.
	StateFailed State = "failed"
	// StateStopped: the supervisor was stopped.
	StateStopped State = "stopped"
	// StateOff: file search is switched off (or not installed); no process runs and none is retried.
	StateOff State = "off"
)

// Status is what the UI shows about the engine.
type Status struct {
	State    State
	Message  string // a reason for Restarting and Failed, otherwise empty
	Restarts int    // how many times the process has been started again
	// Device is where the model runs once Ready: "vulkan", "cuda" or "cpu" (the fallback without a usable graphics card).
	Device string
}

// Options configure a Supervisor.
type Options struct {
	// Dir is the folder the engine runs in (its source).
	Dir string
	// Port is the port to serve on; 0 picks a free one.
	Port int
	// Env is added to the engine's environment (where its llama.cpp server and models are).
	Env []string
	// Log receives everything the engine prints, besides the last lines kept for error messages.
	Log io.Writer
	// OnChange is called after every status change, in the supervisor's goroutine.
	OnChange func(Status)

	// Python is the interpreter of the engine's environment.
	Python string

	// Start, Backoff, Poll and StartupTimeout default to the real thing; tests replace them.
	Start          Starter
	Backoff        func(attempt int) time.Duration
	Poll           time.Duration
	StartupTimeout time.Duration
}

const (
	maxModelFailures = 3  // a model that fails to load this many starts in a row is given up on
	unhealthyPolls   = 3  // consecutive failed health checks of a ready engine before a restart
	tailLines        = 20 // engine output kept for failure messages
	healthTimeout    = 2 * time.Second
	defaultPoll      = 500 * time.Millisecond
	defaultStartup   = 3 * time.Minute
	maxBackoff       = 30 * time.Second
	missingVenvHint  = "turn file search off and on again in Settings to reinstall it"
	needsReindexHint = "the saved index was built with another model, dimension or format and has to be rebuilt (move or delete the memory folder under %LOCALAPPDATA%\\AGI)"
)

// Supervisor starts the engine, waits for it, restarts it when it dies and stops it on request.
type Supervisor struct {
	opts   Options
	token  string
	port   int
	client *Client

	mu     sync.Mutex
	status Status
}

// New prepares a supervisor: it picks the port and generates this run's token.
func New(opts Options) (*Supervisor, error) {
	if opts.Start == nil {
		opts.Start = execStarter
	}
	if opts.Backoff == nil {
		opts.Backoff = defaultBackoff
	}
	if opts.Poll <= 0 {
		opts.Poll = defaultPoll
	}
	if opts.StartupTimeout <= 0 {
		opts.StartupTimeout = defaultStartup
	}
	if opts.Python == "" {
		return nil, errors.New("the engine's python is not set")
	}
	if opts.Log == nil {
		opts.Log = io.Discard
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate the engine token: %w", err)
	}
	port := opts.Port
	if port == 0 {
		var err error
		if port, err = freePort(); err != nil {
			return nil, err
		}
	}
	token := hex.EncodeToString(raw)
	return &Supervisor{
		opts:   opts,
		token:  token,
		port:   port,
		client: NewClient("http://127.0.0.1:"+strconv.Itoa(port), token, nil),
		status: Status{State: StateStarting},
	}, nil
}

// Client returns the client for the engine this supervisor runs. It stays valid across restarts
// (the port and the token are fixed for the supervisor's life).
func (s *Supervisor) Client() *Client { return s.client }

// Status returns the current state.
func (s *Supervisor) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Supervisor) set(state State, message string) {
	s.mu.Lock()
	changed := s.status.State != state || s.status.Message != message
	s.status.State, s.status.Message = state, message
	status := s.status
	s.mu.Unlock()
	if changed && s.opts.OnChange != nil {
		s.opts.OnChange(status)
	}
}

// Off reports that file search is switched off. It is for the Switch, once Run has returned.
func (s *Supervisor) Off() { s.set(StateOff, "") }

func (s *Supervisor) countRestart() {
	s.mu.Lock()
	s.status.Restarts++
	s.mu.Unlock()
}

// Run starts the engine and keeps it running until ctx ends or the engine cannot run (then the
// state is Failed). It stops the process before it returns.
func (s *Supervisor) Run(ctx context.Context) {
	if s.opts.OnChange != nil {
		s.opts.OnChange(s.Status()) // announce the initial state too
	}
	attempt, modelFailures := 0, 0
	for ctx.Err() == nil {
		s.set(StateStarting, "")
		outcome := s.runOnce(ctx, &modelFailures)
		switch outcome.kind {
		case outcomeStopped:
			s.set(StateStopped, "")
			return
		case outcomeFatal:
			s.set(StateFailed, outcome.message)
			return
		}
		s.set(StateRestarting, outcome.message)
		if outcome.wasReady {
			attempt = 0
		}
		select {
		case <-ctx.Done():
			s.set(StateStopped, "")
			return
		case <-time.After(s.opts.Backoff(attempt)):
		}
		attempt++
		s.countRestart()
	}
	s.set(StateStopped, "")
}

type outcomeKind int

const (
	outcomeRetry outcomeKind = iota
	outcomeFatal
	outcomeStopped
)

type outcome struct {
	kind     outcomeKind
	message  string
	wasReady bool
}

func (s *Supervisor) spec() Spec {
	env := append(os.Environ(),
		"AGI_SIDECAR_TOKEN="+s.token,
		"AGI_SIDECAR_PORT="+strconv.Itoa(s.port),
		"PYTHONUTF8=1",
		"PYTHONUNBUFFERED=1",
	)
	env = append(env, s.opts.Env...)
	return Spec{Dir: s.opts.Dir, Exe: s.opts.Python, Args: []string{"-m", "agi_memory"}, Env: env}
}

// runOnce starts the process and watches it until it ends, hangs, or ctx ends.
func (s *Supervisor) runOnce(ctx context.Context, modelFailures *int) outcome {
	output := newTail(tailLines)
	proc, err := s.opts.Start(ctx, s.spec(), io.MultiWriter(output, s.opts.Log))
	if err != nil {
		hint := ""
		if errors.Is(err, fs.ErrNotExist) {
			hint = ": " + missingVenvHint
		}
		return outcome{kind: outcomeFatal, message: "cannot start the search engine" + hint + " (" + err.Error() + ")"}
	}
	exited := make(chan error, 1)
	go func() { exited <- proc.Wait() }()
	stop := func() {
		_ = proc.Kill()
		<-exited
	}

	started := time.Now()
	ticker := time.NewTicker(s.opts.Poll)
	defer ticker.Stop()
	ready, failedPolls := false, 0
	for {
		select {
		case <-ctx.Done():
			stop()
			return outcome{kind: outcomeStopped}
		case err := <-exited:
			return s.afterExit(err, output, ready)
		case <-ticker.C:
		}

		checkCtx, cancel := context.WithTimeout(ctx, healthTimeout)
		health, err := s.client.Health(checkCtx)
		cancel()
		switch {
		case err != nil:
			failedPolls++
			if ready && failedPolls >= unhealthyPolls {
				stop()
				return outcome{message: "the search engine stopped answering", wasReady: true}
			}
		case health.Error != nil:
			stop()
			*modelFailures++
			if *modelFailures >= maxModelFailures {
				return outcome{kind: outcomeFatal, message: "the embedding model cannot be loaded: " + *health.Error}
			}
			return outcome{message: "the embedding model failed to load: " + *health.Error}
		case health.Ready:
			failedPolls, *modelFailures = 0, 0
			ready = true
			if health.Device != nil {
				s.mu.Lock()
				s.status.Device = *health.Device
				s.mu.Unlock()
			}
			s.set(StateReady, "")
		default:
			failedPolls = 0
			s.set(StateWarming, "")
		}
		if !ready && time.Since(started) > s.opts.StartupTimeout {
			stop()
			return outcome{message: "the search engine did not become ready in time"}
		}
	}
}

// afterExit decides what the end of the process means.
func (s *Supervisor) afterExit(err error, output *tail, wasReady bool) outcome {
	last := output.String()
	switch {
	case strings.Contains(last, "reindex required"):
		return outcome{kind: outcomeFatal, message: needsReindexHint}
	case exitCode(err) == 2: // the engine's own "bad configuration" status
		return outcome{kind: outcomeFatal, message: "the search engine refused its settings: " + lastLine(last)}
	}
	message := "the search engine stopped"
	if line := lastLine(last); line != "" {
		message += ": " + line
	}
	return outcome{message: message, wasReady: wasReady}
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// defaultBackoff is 1 s, 2 s, 4 s ... capped at 30 s.
func defaultBackoff(attempt int) time.Duration {
	if attempt > 5 {
		return maxBackoff
	}
	return min(time.Second<<attempt, maxBackoff)
}

// freePort asks the OS for an unused port on the loopback interface.
func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("find a free port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
