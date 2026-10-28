package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"omni/internal/enginesetup"
)

// Progress says where an install is, for the settings page.
type Progress struct {
	Step, Steps int
	Name        string
	Done, Total int64 // bytes of the download in progress
}

// InstallState is an install as the page shows it.
type InstallState struct {
	Installing  bool
	Installed   bool
	NeedsUpdate bool // installed, but from other pins than this version of Omni asks for
	Progress    Progress
	Error       string // why the last install failed; empty after a good one
}

type manifest struct {
	Version     string    `json:"version"` // a hash of the pins
	InstalledAt time.Time `json:"installed_at"`
}

// Setup downloads, checks and removes the model and its server, one install at a time in the background. Every step can
// be repeated: a finished download is kept, a cut one continues.
type Setup struct {
	Download enginesetup.Downloader
	onDone   func(error)

	mu       sync.Mutex
	layout   Layout
	cancel   context.CancelFunc
	running  bool
	progress Progress
	lastErr  string
}

// NewSetup returns a Setup for layout; onDone (may be nil) is called from the install's goroutine when it ends, with its
// error (nil when it worked).
func NewSetup(layout Layout, onDone func(error)) *Setup { return &Setup{layout: layout, onDone: onDone} }

// Layout is where the files are now.
func (s *Setup) Layout() Layout {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.layout
}

// Move says that Omni AI is to be kept somewhere else, as the person chose a folder for the downloads. It refuses once there
// is something to leave behind.
func (s *Setup) Move(to Layout) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.running:
		return errors.New("Omni AI is being installed; the folder cannot change meanwhile")
	case s.layout.Installed():
		return errors.New("Omni AI is already installed in " + s.layout.Root + "; remove it first to put it elsewhere")
	}
	s.layout = to
	return nil
}

func pinsVersion() string {
	sum := sha256.Sum256([]byte(Llama.SHA256 + "\n" + Model.SHA256))
	return hex.EncodeToString(sum[:])[:16]
}

// State reports the install now.
func (s *Setup) State() InstallState {
	s.mu.Lock()
	defer s.mu.Unlock()
	installed := s.layout.Installed()
	return InstallState{
		Installing: s.running, Installed: installed, Progress: s.progress, Error: s.lastErr,
		NeedsUpdate: installed && !s.running && s.installedVersion() != pinsVersion(),
	}
}

func (s *Setup) installedVersion() string {
	data, err := os.ReadFile(s.layout.Manifest())
	var have manifest
	if err != nil || json.Unmarshal(data, &have) != nil {
		return ""
	}
	return have.Version
}

// Start begins an install (or an update, which is the same thing) in the background.
func (s *Setup) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("Omni AI is already being installed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.running, s.cancel, s.progress, s.lastErr = true, cancel, Progress{}, ""
	layout := s.layout
	s.mu.Unlock()
	go func() {
		err := s.install(ctx, layout)
		s.mu.Lock()
		s.running = false
		if err != nil && !errors.Is(err, context.Canceled) {
			s.lastErr = err.Error()
		}
		s.mu.Unlock()
		cancel()
		if s.onDone != nil {
			s.onDone(err)
		}
	}()
	return nil
}

// Cancel stops a running install; what was downloaded is kept for the next one.
func (s *Setup) Cancel() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Setup) report(step int, name string) func(done, total int64) {
	return func(done, total int64) {
		s.mu.Lock()
		s.progress = Progress{Step: step, Steps: 2, Name: name, Done: done, Total: total}
		s.mu.Unlock()
	}
}

func (s *Setup) install(ctx context.Context, l Layout) error {
	if err := os.MkdirAll(l.Root, 0o755); err != nil {
		return fmt.Errorf("create the chat folder: %w", err)
	}
	_ = os.Remove(l.Manifest()) // an install that fails from here on must not look finished
	if err := s.Download.Unpack(ctx, Llama, l.Dir(), s.report(1, "Getting llama.cpp")); err != nil {
		return fmt.Errorf("get llama.cpp: %w", err)
	}
	if err := s.Download.Download(ctx, Model, l.ModelFile(), s.report(2, "Downloading the model")); err != nil {
		return fmt.Errorf("download the model: %w", err)
	}
	for _, p := range []string{l.Server(), l.ModelFile()} {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("%s is missing after the install", filepath.Base(p))
		}
	}
	body, err := json.Marshal(manifest{Version: pinsVersion(), InstalledAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	return os.WriteFile(l.Manifest(), body, 0o644)
}

// Remove deletes the model and its server. It refuses while an install runs; the caller stops the server first, because its
// files are in use.
func (s *Setup) Remove() error {
	s.mu.Lock()
	running, root := s.running, s.layout.Root
	s.mu.Unlock()
	if running {
		return errors.New("Omni AI is being installed; cancel that first")
	}
	if err := enginesetup.RemoveAll(root); err != nil {
		return fmt.Errorf("remove %s: %w", root, err)
	}
	return nil
}
