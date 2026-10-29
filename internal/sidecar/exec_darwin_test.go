package sidecar

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func waitGone(t *testing.T, pid int) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if !alive(pid) {
			return
		}
	}
	t.Fatalf("process %d is still running", pid)
}

// shellTree starts `sh -c "sleep 60 & echo $!; wait"`: a parent that starts a child, the shape of the engine and its llama-server.
func shellTree(t *testing.T) (Process, int) {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	proc, err := ExecStarter(context.Background(), Spec{Exe: "/bin/sh", Args: []string{"-c", "sleep 60 & echo $!; wait"}}, write)
	write.Close()
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 32)
	n, _ := read.Read(buf)
	child, err := strconv.Atoi(strings.TrimSpace(string(buf[:n])))
	if err != nil {
		t.Fatalf("the shell did not report its child: %q", buf[:n])
	}
	return proc, child
}

func TestKillingAProcessKillsWhatItStarted(t *testing.T) {
	proc, child := shellTree(t)
	if err := proc.Kill(); err != nil {
		t.Fatal(err)
	}
	waitGone(t, child)
	_ = proc.Wait()
}

func TestTheReaperKillsWhatItWasToldAboutWhenTheLauncherEnds(t *testing.T) {
	proc, child := shellTree(t)
	defer proc.Kill()
	pgid := proc.(*execProcess).pgid
	read, write, _ := os.Pipe()
	done := make(chan struct{})
	go func() { RunReaper(read); close(done) }()
	_, _ = io.WriteString(write, strconv.Itoa(pgid)+"\n")
	select {
	case <-done:
		t.Fatal("the reaper ended while the launcher was still there")
	case <-time.After(200 * time.Millisecond):
	}
	write.Close() // the launcher ended
	<-done
	waitGone(t, child)
}

func TestTheReaperLeavesAGroupItWasToldEnded(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	read, write, _ := os.Pipe()
	done := make(chan struct{})
	go func() { RunReaper(read); close(done) }()
	_, _ = io.WriteString(write, strconv.Itoa(cmd.Process.Pid)+"\n"+strconv.Itoa(-cmd.Process.Pid)+"\n")
	write.Close()
	<-done
	if !alive(cmd.Process.Pid) {
		t.Fatal("a group that was handed back must not be killed (its number may belong to another process by now)")
	}
}
