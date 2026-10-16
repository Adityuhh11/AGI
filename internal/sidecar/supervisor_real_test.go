//go:build windows && realsidecar

// Run by hand: go test -tags realsidecar -run TestTheRealEngine -v ./internal/sidecar/
// It starts the real Python engine (about 30 s for the model) against the sample folder.

package sidecar

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testLog struct{ t *testing.T }

func (l testLog) Write(p []byte) (int, error) {
	l.t.Log(strings.TrimRight(string(p), "\r\n"))
	return len(p), nil
}

func TestTheRealEngineStartsIndexesSearchesAndStops(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "sidecar"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".venv", "Scripts", "python.exe")); err != nil {
		t.Skip("the sidecar venv is not set up (run `uv sync` in sidecar/)")
	}
	sample, err := filepath.Abs(filepath.Join("..", "..", "Sample Data"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sample); err != nil {
		t.Skip("Sample Data/ is not present")
	}
	t.Setenv("AGI_MEMORY_DIR", t.TempDir())

	sup, err := New(Options{Dir: dir, Log: testLog{t}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() { sup.Run(ctx); close(finished) }()
	started := time.Now()

	waitFor := func(what string, limit time.Duration, done func() bool) {
		t.Helper()
		for deadline := time.Now().Add(limit); time.Now().Before(deadline); time.Sleep(300 * time.Millisecond) {
			if done() {
				return
			}
		}
		t.Fatalf("timed out waiting for %s; status %+v", what, sup.Status())
	}
	waitFor("the engine to be ready", 3*time.Minute, func() bool { return sup.Status().State == StateReady })
	t.Logf("ready after %.1f s", time.Since(started).Seconds())

	client := sup.Client()
	sources, err := client.Sources(ctx)
	if err != nil || len(sources) != 0 {
		t.Fatalf("sources = %v, err = %v; a fresh index should be empty", sources, err)
	}
	jobID, err := client.AddSource(ctx, sample, "sample", nil)
	if err != nil {
		t.Fatal(err)
	}
	var job Job
	waitFor("the index job", 3*time.Minute, func() bool {
		job, err = client.Job(ctx, jobID)
		return err == nil && job.State != JobRunning
	})
	if job.State != JobDone || job.Report == nil || job.Report.ChunksWritten == 0 {
		t.Fatalf("job = %+v", job)
	}

	found, err := client.SearchFiles(ctx, SearchFilesRequest{Query: "how do I refresh the jwt token", K: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(found.Files) == 0 || !strings.HasSuffix(found.Files[0].URI, "jwt.js") || !found.Confident {
		t.Fatalf("search = %+v", found)
	}
	for _, f := range found.Files {
		if f.SourceID != "sample" {
			t.Fatalf("file %s has source %q", f.URI, f.SourceID)
		}
	}

	cancel()
	select {
	case <-finished:
	case <-time.After(15 * time.Second):
		t.Fatal("the supervisor did not stop the engine")
	}
	if _, err := client.Health(context.Background()); err == nil {
		t.Fatal("the engine still answers after the supervisor stopped")
	}
}
