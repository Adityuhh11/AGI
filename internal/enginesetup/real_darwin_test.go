package enginesetup

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// engineSource is what the launcher embeds (pyproject.toml, uv.lock and src/ of the sidecar folder), read from the repository.
func engineSource(t *testing.T) fs.FS {
	t.Helper()
	disk := os.DirFS(filepath.Join("..", "..", "sidecar"))
	out := fstest.MapFS{}
	for _, name := range []string{"pyproject.toml", "uv.lock"} {
		data, err := fs.ReadFile(disk, name)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = &fstest.MapFile{Data: data}
	}
	err := fs.WalkDir(disk, "src", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || skipped(path) {
			return err
		}
		data, err := fs.ReadFile(disk, path)
		out[path] = &fstest.MapFile{Data: data}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestRealInstallOnAMac runs the install as a Mac does it, with the real downloads of uv and llama.cpp and the real Python environment (uv fetches its own Python and the
// libraries, about 110 MB), and then starts what was installed. Only the two models are stand-ins (a gigabyte is not worth fetching to prove the rest).
// It is for the Mac CI job: OMNI_REAL_INSTALL=1.
func TestRealInstallOnAMac(t *testing.T) {
	if os.Getenv("OMNI_REAL_INSTALL") != "1" {
		t.Skip("set OMNI_REAL_INSTALL=1 to download uv, Python and llama.cpp and run them")
	}
	models := newFileServer(t, map[string][]byte{"/text.gguf": payload(1000, 1), "/vision.gguf": payload(1000, 2)})
	installer := NewInstaller(Layout{Root: filepath.Join(t.TempDir(), "engine")}, engineSource(t))
	installer.Pins.Text = artifactOf(models.URL+"/text.gguf", payload(1000, 1))
	installer.Pins.Vision = artifactOf(models.URL+"/vision.gguf", payload(1000, 2))

	ctx, cancel := contextWithTimeout(t, 15*time.Minute)
	defer cancel()
	start := time.Now()
	if err := installer.Install(ctx, func(p Progress) {
		if p.Message != "" {
			t.Log(p.Message)
		}
	}); err != nil {
		t.Fatalf("the install failed: %v", err)
	}
	t.Logf("installed in %v", time.Since(start).Round(time.Second))
	l := installer.Layout
	if !l.Installed() {
		t.Fatal("the install finished but the engine does not count as installed")
	}
	for name, cmd := range map[string]*exec.Cmd{
		"uv":           exec.Command(l.UV(), "--version"),
		"llama-server": exec.Command(l.LlamaServer(), "--version"),
		"python":       exec.Command(l.Python(), "-c", "import sys; print(sys.version)"),
	} {
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("%s does not run: %v\n%s", name, err, out)
			continue
		}
		t.Logf("%s: %s", name, strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]))
	}
}
