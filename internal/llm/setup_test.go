package llm

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"omni/internal/enginesetup"
)

func zipOf(t *testing.T, files map[string]string) []byte {
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

func pin(url string, data []byte) enginesetup.Artifact {
	sum := sha256.Sum256(data)
	return enginesetup.Artifact{Name: "test", URL: url, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}
}

// withPins serves a fake llama.cpp zip and a fake model, points the package's pins at them and gives the test a Setup.
func withPins(t *testing.T) (*Setup, chan error, *atomic.Int32) {
	t.Helper()
	llama := zipOf(t, map[string]string{"llama-server.exe": "server"})
	model := bytes.Repeat([]byte("m"), 50_000)
	var asked atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		switch r.URL.Path {
		case "/llama.zip":
			_, _ = w.Write(llama)
		case "/model.gguf":
			_, _ = w.Write(model)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	oldLlama, oldModel := Llama, Model
	Llama, Model = pin(srv.URL+"/llama.zip", llama), pin(srv.URL+"/model.gguf", model)
	t.Cleanup(func() { Llama, Model = oldLlama, oldModel })
	done := make(chan error, 1)
	return NewSetup(Layout{Root: filepath.Join(t.TempDir(), "chat")}, func(err error) { done <- err }), done, &asked
}

func wait(t *testing.T, done chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("the install did not finish")
		return nil
	}
}

func installed(t *testing.T) (*Setup, chan error, *atomic.Int32) {
	t.Helper()
	s, done, asked := withPins(t)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if err := wait(t, done); err != nil {
		t.Fatal(err)
	}
	return s, done, asked
}

func TestInstallDownloadsBothFilesWritesTheManifestLastAndRepeatsForFree(t *testing.T) {
	s, done, asked := withPins(t)
	if s.State().Installed {
		t.Fatal("nothing is installed yet")
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if err := wait(t, done); err != nil {
		t.Fatal(err)
	}
	st := s.State()
	if !st.Installed || st.NeedsUpdate || st.Error != "" || st.Installing {
		t.Fatalf("state after a good install = %+v", st)
	}
	for _, p := range []string{s.Layout().Server(), s.Layout().ModelFile(), s.Layout().Manifest()} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s missing: %v", p, err)
		}
	}
	before := asked.Load()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if err := wait(t, done); err != nil {
		t.Fatal(err)
	}
	if got := asked.Load(); got != before {
		t.Fatalf("a second install made %d more requests; the checked files should be kept", got-before)
	}
}

func TestAnInstallWithAWrongFileFailsAndDoesNotLookFinished(t *testing.T) {
	s, done, _ := withPins(t)
	Model.SHA256 = "00" + Model.SHA256[2:]
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if err := wait(t, done); err == nil {
		t.Fatal("a download that does not match its checksum was accepted")
	}
	st := s.State()
	if st.Installed || st.Error == "" {
		t.Fatalf("state = %+v, want not installed with a reason", st)
	}
}

func TestChangedPinsMakeAnInstallNeedAnUpdate(t *testing.T) {
	s, _, _ := installed(t)
	Model.SHA256 = "ff" + Model.SHA256[2:]
	if !s.State().NeedsUpdate {
		t.Fatal("an install made from other pins should be offered an update")
	}
}

func TestRemoveDeletesTheFolderAndRefusesWhileInstalling(t *testing.T) {
	s, _, _ := installed(t)
	if err := s.Remove(); err != nil {
		t.Fatal(err)
	}
	if s.State().Installed {
		t.Fatal("still installed after Remove")
	}
	s.running = true
	if err := s.Remove(); err == nil {
		t.Fatal("Remove ran while an install was running")
	}
}

func TestMoveIsRefusedOnceInstalled(t *testing.T) {
	s, done, _ := withPins(t)
	other := Layout{Root: filepath.Join(t.TempDir(), "elsewhere")}
	if err := s.Move(other); err != nil || s.Layout() != other {
		t.Fatalf("Move before an install = %v, layout %v", err, s.Layout())
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if err := wait(t, done); err != nil {
		t.Fatal(err)
	}
	if err := s.Move(Layout{Root: t.TempDir()}); err == nil {
		t.Fatal("Move succeeded with files installed")
	}
}
