//go:build windows

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
		Watch(ctx, []string{root}, 50*time.Millisecond, 500*time.Millisecond, func(r string) { got <- r })
		close(done)
	}()
	time.Sleep(200 * time.Millisecond) // the watch is registered with the operating system
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
	case <-time.After(600 * time.Millisecond):
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
	expectQuiet(t, got, "a burst is one report")

	if err := os.Rename(file, filepath.Join(root, "moved.txt")); err != nil {
		t.Fatal(err)
	}
	expectChange(t, got, root, "a moved file")
	expectQuiet(t, got, "after the move")

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
		Watch(ctx, []string{filepath.Join(good, "does-not-exist"), good}, 50*time.Millisecond, 500*time.Millisecond, func(r string) { got <- r })
		close(done)
	}()
	defer func() { cancel(); <-done }()
	time.Sleep(200 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(good, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectChange(t, got, good, "the folder that exists")
}
