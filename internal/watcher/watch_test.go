package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// start watches root and returns a channel of the changes it reports and a stop function that
// waits for Watch to return.
func start(t *testing.T, root string) (<-chan string, func()) {
	t.Helper()
	got := make(chan string, 16)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Watch(ctx, []string{root}, 200*time.Millisecond, time.Second, func(r string) { got <- r })
		close(done)
	}()
	time.Sleep(500 * time.Millisecond) // the watch is registered with the operating system
	return got, func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("Watch did not return after its context ended")
		}
	}
}

func expectChange(t *testing.T, got <-chan string, root, why string) {
	t.Helper()
	select {
	case r := <-got:
		if r != root {
			t.Fatalf("%s: reported %q, want %q", why, r, root)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("%s: no change reported", why)
	}
}

func expectQuiet(t *testing.T, got <-chan string, why string) {
	t.Helper()
	select {
	case r := <-got:
		t.Fatalf("%s: unexpected change reported for %q", why, r)
	case <-time.After(900 * time.Millisecond):
	}
}

// settle waits out the late events the system sometimes delivers after a change (a folder's own
// modified time, a virus scan touching the file) and drops what they cause. How a burst becomes one
// report is tested on the debouncer, where time is exact.
func settle(got <-chan string) {
	time.Sleep(900 * time.Millisecond)
	for {
		select {
		case <-got:
		default:
			return
		}
	}
}

func TestAFolderInsideAnotherWatchedFolderIsReportedForBoth(t *testing.T) {
	outer := t.TempDir()
	inner := filepath.Join(outer, "Projects")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	got := make(chan string, 16)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Watch(ctx, []string{outer, inner}, 200*time.Millisecond, time.Second, func(r string) { got <- r })
	time.Sleep(1500 * time.Millisecond) // the watches are registered, and the system's look back is over
	if err := os.WriteFile(filepath.Join(inner, "note.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for deadline := time.After(3 * time.Second); len(seen) < 2; {
		select {
		case r := <-got:
			seen[r] = true
		case <-deadline:
			t.Fatalf("reported %v, want both %s and %s", seen, outer, inner)
		}
	}
}

func TestChangesUnderAFolderAreReportedOnceTheyGoQuiet(t *testing.T) {
	root := t.TempDir()
	got, stop := start(t, root)
	defer stop()

	file := filepath.Join(root, "sub", "notes.txt")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectChange(t, got, root, "a new file in a subfolder")
	settle(got)

	if err := os.Rename(file, filepath.Join(root, "moved.txt")); err != nil {
		t.Fatal(err)
	}
	expectChange(t, got, root, "a moved file")
	settle(got)

	if err := os.Remove(filepath.Join(root, "moved.txt")); err != nil {
		t.Fatal(err)
	}
	expectChange(t, got, root, "a deleted file")
}

func TestNoiseIsNotReported(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "app"), 0o755); err != nil { // a new folder would count
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond) // FSEvents also reports what happened just before it started listening
	got, stop := start(t, root)
	defer stop()
	for _, name := range []string{
		filepath.Join(".git", "index"),
		filepath.Join("app", "node_modules", "react", "index.js"),
		"download.crdownload",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	expectQuiet(t, got, "files the engine never indexes")
}

func TestAFolderThatCannotBeWatchedDoesNotStopTheOthers(t *testing.T) {
	good := t.TempDir()
	got := make(chan string, 4)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Watch(ctx, []string{filepath.Join(good, "does-not-exist"), good}, 200*time.Millisecond, time.Second, func(r string) { got <- r })
		close(done)
	}()
	defer func() { cancel(); <-done }()
	time.Sleep(500 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(good, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectChange(t, got, good, "the folder that exists")
}

// A context that ends before the operating system has been asked to wait must still end the watch: the cancelling call
// that arrives too early finds nothing to cancel, and the wait that starts after it must not be left to a file change.
func TestWatchReturnsWhenItsContextEndedBeforeItStarted(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		done := make(chan struct{})
		go func() {
			Watch(ctx, []string{root}, 200*time.Millisecond, time.Second, func(string) {})
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatalf("Watch did not return for a context that had already ended (attempt %d)", i)
		}
	}
}
