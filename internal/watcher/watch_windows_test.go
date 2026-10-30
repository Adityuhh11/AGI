//go:build windows

package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAFolderThatAppearsLaterIsWatchedAndReportedOnce(t *testing.T) {
	retryAfter = 100 * time.Millisecond
	t.Cleanup(func() { retryAfter = 30 * time.Second })
	root := filepath.Join(t.TempDir(), "usb")
	got := make(chan string, 16)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Watch(ctx, []string{root}, 100*time.Millisecond, time.Second, func(r string) { got <- r })
		close(done)
	}()
	time.Sleep(300 * time.Millisecond) // it has failed to open the folder at least once
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	expectChange(t, got, root, "the folder was plugged in")
	settle(got)
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectChange(t, got, root, "a file added after it came back")
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Watch did not return after its context ended")
	}
}
