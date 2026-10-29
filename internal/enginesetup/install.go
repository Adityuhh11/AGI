package enginesetup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Pins are the files an install downloads.
type Pins struct{ UV, Llama, Text, Vision Artifact }

// DefaultPins are the artifacts measured in docs/eval/2026-10-18-llamacpp-embedder.md.
func DefaultPins() Pins { return Pins{UV: UV, Llama: Llama, Text: TextModel, Vision: VisionModel} }

// Progress says where an install is, for the settings window.
type Progress struct {
	Step, Steps int
	Name        string // what this step does: "Downloading the text model"
	Done, Total int64  // bytes, while a download runs (Total is 0 when there is no figure)
	Message     string // the last line uv printed during the long step
}

// Manifest records a finished install; it is written last.
type Manifest struct {
	Version     string    `json:"version"` // see Installer.Version
	InstalledAt time.Time `json:"installed_at"`
}

// Installer puts the engine in its folder: uv, the engine's source, Python and the libraries, llama.cpp and
// the model files. Every step can be repeated: a finished download is checked and kept, a cut one continues.
type Installer struct {
	Layout   Layout
	Source   fs.FS // the engine's source: pyproject.toml, uv.lock and src/ (the repository's sidecar folder)
	Pins     Pins
	Download Downloader

	// RunUV runs uv in dir with the environment env; Check proves the installed environment can import the
	// engine. Tests replace both.
	RunUV func(ctx context.Context, dir string, env, args []string, output io.Writer) error
	Check func(ctx context.Context) error
}

// NewInstaller returns an installer for layout that installs the engine whose source is source.
func NewInstaller(layout Layout, source fs.FS) *Installer {
	return &Installer{Layout: layout, Source: source, Pins: DefaultPins()}
}

// Version names what this installer would install: a hash of the engine's source and of the pinned
// downloads. An installed engine with another version is out of date.
func (i *Installer) Version() (string, error) {
	sum := sha256.New()
	var paths []string
	err := fs.WalkDir(i.Source, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || skipped(path) {
			return err
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("read the engine source: %w", err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		data, err := fs.ReadFile(i.Source, path)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(sum, "%s\n%d\n", path, len(data))
		sum.Write(data)
	}
	for _, pin := range []Artifact{i.Pins.UV, i.Pins.Llama, i.Pins.Text, i.Pins.Vision} {
		fmt.Fprintf(sum, "%s\n", pin.SHA256)
	}
	return hex.EncodeToString(sum.Sum(nil))[:16], nil
}

// NeedsUpdate reports whether an installed engine differs from what this installer would install.
func (i *Installer) NeedsUpdate() bool {
	if !i.Layout.Installed() {
		return false
	}
	want, err := i.Version()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(i.Layout.Manifest())
	var have Manifest
	return err != nil || json.Unmarshal(data, &have) != nil || have.Version != want
}

// skipped is a file that is not part of the engine's source (Python's compiled caches).
func skipped(path string) bool {
	return strings.Contains(path, "__pycache__") || strings.HasSuffix(path, ".pyc")
}

type step struct {
	name string
	run  func(ctx context.Context, report func(Progress)) error
}

// Install runs every step in order and, if all of them worked, writes the manifest. report is called as
// the install moves; ctx cancels it, leaving what was downloaded for the next attempt.
func (i *Installer) Install(ctx context.Context, report func(Progress)) error {
	if report == nil {
		report = func(Progress) {}
	}
	version, err := i.Version()
	if err != nil {
		return err
	}
	l := i.Layout
	steps := []step{
		{"Getting the package tool", func(ctx context.Context, r func(Progress)) error {
			return i.unpacked(ctx, i.Pins.UV, filepath.Join(l.Root, "tools"), r)
		}},
		{"Unpacking the search engine", func(context.Context, func(Progress)) error { return writeSource(i.Source, l.App()) }},
		{"Installing Python and its libraries", i.syncPython},
		{"Getting llama.cpp", func(ctx context.Context, r func(Progress)) error {
			return i.unpacked(ctx, i.Pins.Llama, filepath.Join(l.Root, "llama"), r)
		}},
		{"Downloading the models", i.models},
		{"Checking that it starts", func(ctx context.Context, _ func(Progress)) error { return i.check(ctx) }},
	}
	if err := os.MkdirAll(l.Root, 0o755); err != nil {
		return fmt.Errorf("create the engine folder: %w", err)
	}
	_ = os.Remove(l.Manifest()) // an install that fails from here on must not look finished
	for n, s := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		at := func(p Progress) { p.Step, p.Steps, p.Name = n+1, len(steps), s.name; report(p) }
		at(Progress{})
		if err := s.run(ctx, at); err != nil {
			return fmt.Errorf("%s: %w", strings.ToLower(s.name[:1])+s.name[1:], err)
		}
	}
	body, err := json.Marshal(Manifest{Version: version, InstalledAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	return os.WriteFile(l.Manifest(), body, 0o644)
}

// unpacked downloads a pinned zip and extracts it into dir, once.
func (i *Installer) unpacked(ctx context.Context, a Artifact, dir string, report func(Progress)) error {
	return i.Download.Unpack(ctx, a, dir, func(done, total int64) { report(Progress{Done: done, Total: total}) })
}

// Unpack downloads a pinned zip and extracts it into dir, once: a marker file in dir records which
// pin is there, so a repeat does not download it again.
func (d Downloader) Unpack(ctx context.Context, a Artifact, dir string, progress func(done, total int64)) error {
	marker := filepath.Join(dir, ".pin")
	if data, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(data)) == a.SHA256 {
		return nil
	}
	archive := dir + ".zip"
	if err := d.Download(ctx, a, archive, progress); err != nil {
		return err
	}
	if err := removeAll(dir); err != nil {
		return fmt.Errorf("clear %s: %w", dir, err)
	}
	if err := Unzip(archive, dir); err != nil {
		return err
	}
	if err := os.WriteFile(marker, []byte(a.SHA256), 0o644); err != nil {
		return err
	}
	return os.Remove(archive)
}

// models downloads the two model files; the figures run across both.
func (i *Installer) models(ctx context.Context, report func(Progress)) error {
	files := []Artifact{i.Pins.Text, i.Pins.Vision}
	total := files[0].Size + files[1].Size
	var before int64
	for _, a := range files {
		offset := before
		dest := filepath.Join(i.Layout.Models(), filepath.Base(a.URL))
		if err := i.Download.Download(ctx, a, dest, func(done, _ int64) { report(Progress{Done: offset + done, Total: total}) }); err != nil {
			return err
		}
		before += a.Size
	}
	return nil
}

// syncPython has uv install its own Python and the engine's libraries into the engine folder. The Python
// version is pinned and uv is told to use only its own, so the install does not depend on, or disturb,
// whatever Python the person has.
func (i *Installer) syncPython(ctx context.Context, report func(Progress)) error {
	l := i.Layout
	cache := filepath.Join(l.Root, "uv-cache")
	env := append(os.Environ(),
		"UV_CACHE_DIR="+cache,
		"UV_PYTHON_INSTALL_DIR="+filepath.Join(l.Root, "python"),
		"UV_PROJECT_ENVIRONMENT="+filepath.Join(l.Root, "venv"),
		"UV_PYTHON_PREFERENCE=only-managed",
		"UV_NO_PROGRESS=1",
	)
	out := &lastLines{report: func(line string) { report(Progress{Message: line}) }}
	run := i.RunUV
	if run == nil {
		run = i.defaultUV
	}
	if err := run(ctx, l.App(), env, []string{"sync", "--frozen", "--no-dev", "--python", PythonVersion}, out); err != nil {
		tail := out.tail()
		if strings.Contains(tail, "os error 448") { // Windows refused a link in the folder; another folder usually has none
			return fmt.Errorf("%w\n%s\nWindows would not let Python be set up in this folder. Choose another one with Change folder (for example C:\\OmniData) and try again", err, tail)
		}
		return fmt.Errorf("%w\n%s", err, tail)
	}
	return removeAll(cache) // the wheels are in the environment now; keep the folder small
}

func (i *Installer) defaultUV(ctx context.Context, dir string, env, args []string, output io.Writer) error {
	cmd := exec.CommandContext(ctx, i.Layout.UV(), args...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, env, output, output
	hide(cmd)
	return cmd.Run()
}

func (i *Installer) check(ctx context.Context) error {
	if i.Check != nil {
		return i.Check(ctx)
	}
	cmd := exec.CommandContext(ctx, i.Layout.Python(), "-c", "import omni_memory, av, fastapi, numpy, PIL, pypdfium2, uvicorn")
	cmd.Dir = i.Layout.App()
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	hide(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(output.String()))
	}
	for _, path := range []string{i.Layout.LlamaServer(), filepath.Join(i.Layout.Models(), filepath.Base(i.Pins.Text.URL)), filepath.Join(i.Layout.Models(), filepath.Base(i.Pins.Vision.URL))} {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("%s is missing", path)
		}
	}
	return nil
}

// writeSource replaces dir with the engine's source files, leaving out compiled caches.
func writeSource(source fs.FS, dir string) error {
	if err := removeAll(dir); err != nil {
		return fmt.Errorf("clear %s: %w", dir, err)
	}
	return fs.WalkDir(source, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || skipped(path) {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// Remove deletes the engine folder (the index lives elsewhere and stays).
func (i *Installer) Remove() error {
	if err := removeAll(i.Layout.Root); err != nil {
		return fmt.Errorf("remove %s: %w", i.Layout.Root, err)
	}
	return nil
}

// RemoveAll deletes dir the way Remove does, for the other installers that keep their files in a folder of their own.
func RemoveAll(dir string) error { return removeAll(dir) }

// removeAll deletes dir, trying again for a few seconds: on Windows a process that has just been killed
// still holds its files for a moment, and deleting them at once fails with a sharing violation.
func removeAll(dir string) error {
	var err error
	for range 20 {
		if err = os.RemoveAll(dir); err == nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return err
}

// lastLines passes each line of uv's output on and remembers the last few for an error message.
type lastLines struct {
	report  func(line string)
	partial string
	lines   []string
}

func (l *lastLines) Write(p []byte) (int, error) {
	l.partial += string(p)
	for {
		end := strings.IndexByte(l.partial, '\n')
		if end < 0 {
			return len(p), nil
		}
		if line := strings.TrimSpace(l.partial[:end]); line != "" {
			l.lines = append(l.lines, line)
			if len(l.lines) > 12 {
				l.lines = l.lines[1:]
			}
			l.report(line)
		}
		l.partial = l.partial[end+1:]
	}
}

func (l *lastLines) tail() string { return strings.Join(l.lines, "\n") }

// State is an install as the window shows it.
type State struct {
	Installing  bool
	Installed   bool
	NeedsUpdate bool
	Progress    Progress
	Error       string // why the last install failed; empty after a good one
}

// Manager runs one install at a time in the background and says how it is going.
type Manager struct {
	installer *Installer
	onDone    func(error)

	mu       sync.Mutex
	cancel   context.CancelFunc
	running  bool
	progress Progress
	lastErr  string
}

// NewManager returns a manager; onDone (may be nil) is called, from the install's goroutine, when an
// install ends, with its error (nil when it worked).
func NewManager(i *Installer, onDone func(error)) *Manager {
	return &Manager{installer: i, onDone: onDone}
}

// Installed reports whether a finished install is there.
func (m *Manager) Installed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.installer.Layout.Installed()
}

// Layout is where the engine and the index are now.
func (m *Manager) Layout() Layout {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.installer.Layout
}

// Move says that the engine and the index are to live somewhere else. It refuses once there is something to leave
// behind (an install running, an engine installed, an index built): moving what exists is not done, only choosing where
// a first install goes.
func (m *Manager) Move(to Layout) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case m.running:
		return errors.New("file search is being installed; the folder cannot change meanwhile")
	case m.installer.Layout.Installed():
		return errors.New("file search is already installed in " + m.installer.Layout.Root + "; delete its files first to put it elsewhere")
	case m.installer.Layout.HasIndex():
		return errors.New("an index is already built in " + m.installer.Layout.Index() + "; it would be left behind")
	}
	m.installer.Layout = to
	return nil
}

// Start begins an install (or a repair or update, which is the same thing) in the background.
func (m *Manager) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return errors.New("file search is already being installed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.running, m.cancel, m.progress, m.lastErr = true, cancel, Progress{}, ""
	m.mu.Unlock()
	go func() {
		err := m.installer.Install(ctx, func(p Progress) {
			m.mu.Lock()
			if p.Step != m.progress.Step {
				m.progress = p // a new step starts afresh
			} else {
				if p.Total > 0 {
					m.progress.Done, m.progress.Total = p.Done, p.Total
				}
				if p.Message != "" {
					m.progress.Message = p.Message
				}
			}
			m.mu.Unlock()
		})
		m.mu.Lock()
		m.running = false
		if err != nil && !errors.Is(err, context.Canceled) {
			m.lastErr = err.Error()
		}
		m.mu.Unlock()
		cancel()
		if m.onDone != nil {
			m.onDone(err)
		}
	}()
	return nil
}

// Cancel stops a running install; what was downloaded is kept for the next one.
func (m *Manager) Cancel() {
	m.mu.Lock()
	cancel := m.cancel
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// State reports the install now.
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return State{
		Installing: m.running, Installed: m.installer.Layout.Installed(),
		NeedsUpdate: !m.running && m.installer.NeedsUpdate(), Progress: m.progress, Error: m.lastErr,
	}
}

// RemoveIndex deletes the index the engine built. The engine must not be running: it holds the files open.
func (m *Manager) RemoveIndex() error {
	m.mu.Lock()
	dir := m.installer.Layout.Index()
	m.mu.Unlock()
	if dir == "" {
		return nil
	}
	if err := removeAll(dir); err != nil {
		return fmt.Errorf("remove %s: %w", dir, err)
	}
	return nil
}

// Remove deletes the installed engine. It refuses while an install runs.
func (m *Manager) Remove() error {
	m.mu.Lock()
	running := m.running
	m.mu.Unlock()
	if running {
		return errors.New("file search is being installed; cancel that first")
	}
	return m.installer.Remove()
}
