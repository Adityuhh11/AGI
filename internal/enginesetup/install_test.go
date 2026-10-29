package enginesetup

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		f, _ := w.Create(name)
		_, _ = f.Write([]byte(body))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// rig is an installer wired to a local server and a fake uv that "installs" by writing the interpreter.
type rig struct {
	*Installer
	srv      *fileServer
	source   fstest.MapFS
	uvCalls  [][]string
	uvEnv    []string
	uvFail   error
	reports  []Progress
	textData []byte
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{source: fstest.MapFS{
		"pyproject.toml":                    {Data: []byte("[project]\nname='x'\n")},
		"uv.lock":                           {Data: []byte("lock")},
		"src/omni_memory/__init__.py":        {Data: []byte("")},
		"src/omni_memory/__pycache__/a.pyc":  {Data: []byte("junk")},
		"src/omni_memory/embedding/llama.py": {Data: []byte("code")},
	}}
	uvZip := zipBytes(t, map[string]string{filepath.Base(Layout{}.UV()): "uv"})
	llamaZip := zipBytes(t, map[string]string{filepath.Base(Layout{}.LlamaServer()): "server", "ggml.dll": "dll"})
	r.textData = payload(300_000, 11)
	vision := payload(500_000, 12)
	r.srv = newFileServer(t, map[string][]byte{"/uv.zip": uvZip, "/llama.zip": llamaZip, "/text.gguf": r.textData, "/vision.gguf": vision})
	layout := Layout{Root: filepath.Join(t.TempDir(), "engine")}
	r.Installer = NewInstaller(layout, r.source)
	r.Pins = Pins{
		UV:     artifactOf(r.srv.URL+"/uv.zip", uvZip),
		Llama:  artifactOf(r.srv.URL+"/llama.zip", llamaZip),
		Text:   artifactOf(r.srv.URL+"/text.gguf", r.textData),
		Vision: artifactOf(r.srv.URL+"/vision.gguf", vision),
	}
	r.RunUV = func(_ context.Context, dir string, env, args []string, out io.Writer) error {
		r.uvCalls, r.uvEnv = append(r.uvCalls, args), env
		_, _ = io.WriteString(out, "Downloading cpython-3.12 (23.3MiB)\nInstalling 34 packages\n")
		if r.uvFail != nil {
			_, _ = io.WriteString(out, "error: no matching distribution\n")
			return r.uvFail
		}
		python := layout.Python()
		if err := os.MkdirAll(filepath.Dir(python), 0o755); err != nil {
			return err
		}
		return os.WriteFile(python, nil, 0o755)
	}
	r.Check = func(context.Context) error { return nil }
	return r
}

func (r *rig) install(ctx context.Context) error {
	return r.Install(ctx, func(p Progress) { r.reports = append(r.reports, p) })
}

func TestAnInstallPutsEverythingWhereTheEngineExpectsItAndWritesTheManifestLast(t *testing.T) {
	r := newRig(t)
	if err := r.install(t.Context()); err != nil {
		t.Fatal(err)
	}
	l := r.Layout
	if !l.Installed() {
		t.Fatal("the install finished but the engine does not count as installed")
	}
	for _, path := range []string{
		l.UV(), l.LlamaServer(), filepath.Join(l.Root, "llama", "ggml.dll"),
		filepath.Join(l.Models(), "text.gguf"), filepath.Join(l.Models(), "vision.gguf"),
		filepath.Join(l.App(), "pyproject.toml"), filepath.Join(l.App(), "uv.lock"),
		filepath.Join(l.App(), "src", "omni_memory", "embedding", "llama.py"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(l.App(), "src", "omni_memory", "__pycache__")); !os.IsNotExist(err) {
		t.Error("compiled caches were copied into the engine")
	}
	for _, leftover := range []string{filepath.Join(l.Root, "uv-cache"), filepath.Join(l.Root, "tools.zip"), filepath.Join(l.Root, "llama.zip")} {
		if _, err := os.Stat(leftover); !os.IsNotExist(err) {
			t.Errorf("%s was left behind", leftover)
		}
	}
}

func TestUvIsToldToUseItsOwnPinnedPythonAndTheEngineFolder(t *testing.T) {
	r := newRig(t)
	if err := r.install(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.uvCalls[0], " "); got != "sync --frozen --no-dev --python 3.12" {
		t.Fatalf("uv ran %q", got)
	}
	env := strings.Join(r.uvEnv, "\n")
	for _, want := range []string{
		"UV_PYTHON_PREFERENCE=only-managed", "UV_PROJECT_ENVIRONMENT=" + filepath.Join(r.Layout.Root, "venv"),
		"UV_PYTHON_INSTALL_DIR=" + filepath.Join(r.Layout.Root, "python"), "UV_CACHE_DIR=" + filepath.Join(r.Layout.Root, "uv-cache"),
	} {
		if !strings.Contains(env, want) {
			t.Errorf("uv's environment lacks %s", want)
		}
	}
}

func TestProgressNamesTheStepsInOrderAndCountsBytesAcrossBothModels(t *testing.T) {
	r := newRig(t)
	if err := r.install(t.Context()); err != nil {
		t.Fatal(err)
	}
	seen := map[int]string{}
	var lastModelDone, modelTotal int64
	for _, p := range r.reports {
		seen[p.Step] = p.Name
		if p.Steps != 6 {
			t.Fatalf("Steps = %d", p.Steps)
		}
		if p.Step == 5 && p.Total > 0 {
			if p.Done < lastModelDone {
				t.Fatalf("model progress went back from %d to %d", lastModelDone, p.Done)
			}
			lastModelDone, modelTotal = p.Done, p.Total
		}
	}
	if len(seen) != 6 || seen[1] != "Getting the package tool" || seen[5] != "Downloading the models" {
		t.Fatalf("steps seen: %v", seen)
	}
	if want := r.Pins.Text.Size + r.Pins.Vision.Size; modelTotal != want || lastModelDone != want {
		t.Fatalf("model progress ended at %d of %d, want %d", lastModelDone, modelTotal, want)
	}
	found := false
	for _, p := range r.reports {
		found = found || strings.Contains(p.Message, "Installing 34 packages")
	}
	if !found {
		t.Fatal("what uv printed was not passed on")
	}
}

func TestInstallingAgainDownloadsNothingAndKeepsTheManifestValid(t *testing.T) {
	r := newRig(t)
	if err := r.install(t.Context()); err != nil {
		t.Fatal(err)
	}
	before := len(r.srv.requests())
	if err := r.install(t.Context()); err != nil {
		t.Fatal(err)
	}
	if after := len(r.srv.requests()); after != before {
		t.Fatalf("a repeat install made %d more requests", after-before)
	}
	if r.NeedsUpdate() || !r.Layout.Installed() {
		t.Fatal("a repeated install left the engine out of date or uninstalled")
	}
}

func TestAFailedStepLeavesNoManifestAndSaysWhatUvPrinted(t *testing.T) {
	r := newRig(t)
	r.uvFail = errors.New("exit status 2")
	err := r.install(t.Context())
	if err == nil || !strings.Contains(err.Error(), "installing Python and its libraries") ||
		!strings.Contains(err.Error(), "no matching distribution") {
		t.Fatalf("Install error = %v", err)
	}
	if r.Layout.Installed() {
		t.Fatal("a failed install counts as installed")
	}
	if _, statErr := os.Stat(r.Layout.UV()); statErr != nil {
		t.Fatal("the steps before the failure were thrown away")
	}
	r.uvFail = nil
	before := len(r.srv.requests())
	if err := r.install(t.Context()); err != nil {
		t.Fatal(err)
	}
	if n := len(r.srv.requests()) - before; n != 4-1 { // uv was kept; llama and the two models are still to fetch
		t.Fatalf("the retry made %d requests, want 3", n)
	}
}

func TestACheckThatFailsFailsTheInstall(t *testing.T) {
	r := newRig(t)
	r.Check = func(context.Context) error { return errors.New("ImportError: no module named omni_memory") }
	err := r.install(t.Context())
	if err == nil || !strings.Contains(err.Error(), "checking that it starts") {
		t.Fatalf("Install error = %v", err)
	}
	if r.Layout.Installed() {
		t.Fatal("an engine that cannot import counts as installed")
	}
}

func TestCancellingLeavesWhatWasDownloadedAndTheNextInstallFinishesTheJob(t *testing.T) {
	r := newRig(t)
	r.srv.block, r.srv.blockPath = make(chan struct{}), "/text.gguf" // the models never arrive
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- r.install(ctx) }()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		for _, req := range r.srv.requests() {
			if strings.HasPrefix(req, "/text.gguf") {
				cancel()
				deadline = time.Now()
			}
		}
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Install error = %v, want a cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the install did not stop when cancelled")
	}
	if r.Layout.Installed() {
		t.Fatal("a cancelled install counts as installed")
	}
	r.srv.block = nil
	if err := r.install(t.Context()); err != nil {
		t.Fatalf("the install after a cancel failed: %v", err)
	}
	if !r.Layout.Installed() {
		t.Fatal("not installed after the second try")
	}
}

func TestANewEngineSourceMakesTheInstalledOneOutOfDate(t *testing.T) {
	r := newRig(t)
	if err := r.install(t.Context()); err != nil {
		t.Fatal(err)
	}
	if r.NeedsUpdate() {
		t.Fatal("a fresh install is out of date")
	}
	r.source["src/omni_memory/embedding/llama.py"] = &fstest.MapFile{Data: []byte("new code")}
	if !r.NeedsUpdate() {
		t.Fatal("changed engine code did not make the installed engine out of date")
	}
	if err := r.install(t.Context()); err != nil {
		t.Fatal(err)
	}
	if r.NeedsUpdate() {
		t.Fatal("still out of date after updating")
	}
	if got, _ := os.ReadFile(filepath.Join(r.Layout.App(), "src", "omni_memory", "embedding", "llama.py")); string(got) != "new code" {
		t.Fatalf("the engine's code was not replaced: %q", got)
	}
}

func TestRemoveDeletesTheEngineFolder(t *testing.T) {
	r := newRig(t)
	if err := r.install(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := r.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(r.Layout.Root); !os.IsNotExist(err) || r.Layout.Installed() {
		t.Fatalf("the engine folder is still there: %v", err)
	}
}

func TestTheManagerInstallsInTheBackgroundOneAtATimeAndReportsDone(t *testing.T) {
	r := newRig(t)
	var mu sync.Mutex
	var finished []error
	m := NewManager(r.Installer, func(err error) { mu.Lock(); finished = append(finished, err); mu.Unlock() })
	r.srv.block, r.srv.blockPath = make(chan struct{}), "/text.gguf"
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err == nil {
		t.Fatal("a second install was started while the first runs")
	}
	if st := m.State(); !st.Installing || st.Installed {
		t.Fatalf("state during the install: %+v", st)
	}
	if err := m.Remove(); err == nil {
		t.Fatal("the engine could be removed while it was being installed")
	}
	close(r.srv.block)
	for deadline := time.Now().Add(10 * time.Second); m.State().Installing && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
	}
	st := m.State()
	mu.Lock()
	defer mu.Unlock()
	if st.Installing || !st.Installed || st.Error != "" || len(finished) != 1 || finished[0] != nil {
		t.Fatalf("state after the install: %+v, done calls: %v", st, finished)
	}
}

func TestTheManagerReportsAFailureAndCancelStopsAnInstall(t *testing.T) {
	r := newRig(t)
	r.uvFail = errors.New("exit status 2")
	m := NewManager(r.Installer, nil)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); m.State().Installing && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
	}
	if st := m.State(); st.Installing || st.Installed || !strings.Contains(st.Error, "no matching distribution") {
		t.Fatalf("state after a failure: %+v", st)
	}

	r.uvFail = nil
	r.srv.block, r.srv.blockPath = make(chan struct{}), "/text.gguf"
	if err := m.Start(); err != nil {
		t.Fatalf("a new install after a failure was refused: %v", err)
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		asked := strings.Join(r.srv.requests(), " ")
		if strings.Contains(asked, "/text.gguf") {
			break
		}
	}
	m.Cancel()
	for deadline := time.Now().Add(10 * time.Second); m.State().Installing && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
	}
	if st := m.State(); st.Installing || st.Error != "" {
		t.Fatalf("state after a cancel: %+v (a cancel is not an error to show)", st)
	}
}

func TestTheHostsAndTheSizeAreWhatThePinsSay(t *testing.T) {
	if got := DownloadBytes(); got < 900<<20 || got > 1100<<20 {
		t.Fatalf("DownloadBytes = %d MB, want about 1 GB", got>>20)
	}
	for _, art := range []Artifact{UV, Llama, TextModel, VisionModel} {
		host := art.URL[len("https://"):strings.Index(art.URL[len("https://"):], "/")+len("https://")]
		found := false
		for _, h := range Hosts {
			found = found || h == host
		}
		if !found {
			t.Errorf("%s is fetched from %s, which Hosts does not list", art.Name, host)
		}
	}
}

func TestTheFolderCanChangeOnlyBeforeThereIsAnythingInIt(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir()) // the default index folder must not be this computer's
	r := newRig(t)
	m := NewManager(r.Installer, nil)
	elsewhere := Layout{Root: filepath.Join(t.TempDir(), "engine"), Memory: filepath.Join(t.TempDir(), "memory")}
	if err := m.Move(elsewhere); err != nil || m.Layout() != elsewhere {
		t.Fatalf("a fresh install can be put anywhere: %v, layout %+v", err, m.Layout())
	}
	touch(t, filepath.Join(elsewhere.Memory, "meta.db"))
	if err := m.Move(Layout{Root: t.TempDir()}); err == nil || !strings.Contains(err.Error(), "index") {
		t.Fatalf("an index would be left behind: %v", err)
	}
	if err := os.Remove(filepath.Join(elsewhere.Memory, "meta.db")); err != nil {
		t.Fatal(err)
	}
	touch(t, elsewhere.Python())
	touch(t, elsewhere.Manifest())
	if err := m.Move(Layout{Root: t.TempDir()}); err == nil || !strings.Contains(err.Error(), "installed") {
		t.Fatalf("an installed engine cannot be moved: %v", err)
	}
}
