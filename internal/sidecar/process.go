package sidecar

import (
	"context"
	"io"
	"strings"
	"sync"
)

// Spec says how to start the engine process.
type Spec struct {
	Dir  string   // working directory
	Exe  string   // program to run
	Args []string // its arguments
	Env  []string // the complete environment, as KEY=value
}

// Process is a started engine process.
type Process interface {
	// Wait blocks until the process ends and returns nil for a clean exit; an error that has an
	// ExitCode() int method reports the exit status.
	Wait() error
	// Kill ends the process.
	Kill() error
}

// Starter starts the engine and sends its output (stdout and stderr) to output.
type Starter func(ctx context.Context, spec Spec, output io.Writer) (Process, error)

// exitCode returns the exit status carried by err, or -1 if it carries none.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if coded, ok := err.(interface{ ExitCode() int }); ok {
		return coded.ExitCode()
	}
	return -1
}

// tail keeps the last lines written to it, so a failure can say what the engine printed.
type tail struct {
	mu      sync.Mutex
	partial string
	lines   []string
	max     int
}

func newTail(maxLines int) *tail { return &tail{max: maxLines} }

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

// Lines returns the last lines seen, oldest first (a line still being written is included).
func (t *tail) Lines() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := append([]string(nil), t.lines...)
	if rest := strings.TrimSpace(t.partial); rest != "" {
		out = append(out, rest)
	}
	return out
}

func (t *tail) String() string { return strings.Join(t.Lines(), "\n") }
