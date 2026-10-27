//go:build windows

package sidecar

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestHelperProcess is not a test: the tests below start this binary as a parent that starts it again as a
// child, the shape of the engine (launcher, interpreter, llama-server).
func TestHelperProcess(t *testing.T) {
	switch os.Getenv("OMNI_HELPER") {
	case "parent":
		child := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
		child.Env = append(os.Environ(), "OMNI_HELPER=child")
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(os.Getenv("OMNI_HELPER_PID"), []byte(strconv.Itoa(child.Process.Pid)), 0o644)
		_ = child.Wait()
	case "child":
		time.Sleep(time.Minute)
	}
}

func running(pid int) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	event, _ := windows.WaitForSingleObject(handle, 0)
	return event == uint32(windows.WAIT_TIMEOUT)
}

func TestKillingAProcessKillsWhatItStarted(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	proc, err := execStarter(context.Background(), Spec{
		Exe:  os.Args[0],
		Args: []string{"-test.run=TestHelperProcess"},
		Env:  append(os.Environ(), "OMNI_HELPER=parent", "OMNI_HELPER_PID="+pidFile),
	}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var child int
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if data, err := os.ReadFile(pidFile); err == nil && len(data) > 0 {
			child, _ = strconv.Atoi(string(data))
			break
		}
	}
	if child == 0 || !running(child) {
		t.Fatalf("the helper's child did not start (pid %d)", child)
	}
	_ = proc.Kill()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if !running(child) {
			return
		}
	}
	t.Fatal("the child outlived the process that started it")
}
