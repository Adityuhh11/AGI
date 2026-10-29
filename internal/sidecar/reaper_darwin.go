package sidecar

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
)

// ReaperFlag makes the program run as the reaper (RunReaper) and nothing else; cmd/omni checks for it before anything starts.
const ReaperFlag = "--reaper"

// macOS has nothing like Windows' Job Object, which kills the engine whenever the launcher ends however it ends (a crash, the
// Activity Monitor's Force Quit). What it does have is that a pipe closes when the process holding its write end is gone. So the
// launcher starts a second copy of itself as the reaper and keeps the write end of a pipe to it. Each program the launcher starts
// gets a process group of its own (so the engine's llama-server is in it too), and the launcher tells the reaper the group's number.
// When the launcher is gone the reaper reads the end of the pipe and kills every group it was told about.

var (
	reaperMu   sync.Mutex
	reaperPipe io.WriteCloser // the launcher's end; never closed, so it lives exactly as long as the launcher
)

// KillWithLauncher starts the reaper. After it, every program ExecStarter starts is killed if the launcher ends.
func KillWithLauncher() error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate this program: %w", err)
	}
	cmd := exec.Command(self, ReaperFlag)
	// The reaper must not die with the launcher's own group before it has done its work; its own group keeps it out of a terminal's Ctrl+C.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start the reaper: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	reaperMu.Lock()
	reaperPipe = pipe
	reaperMu.Unlock()
	return nil
}

// watch tells the reaper about a process group, if there is a reaper.
func watch(pgid int) {
	reaperMu.Lock()
	defer reaperMu.Unlock()
	if reaperPipe != nil {
		_, _ = fmt.Fprintln(reaperPipe, pgid)
	}
}

// forget tells the reaper a group ended by itself, so a later process that gets the same number is never killed by mistake.
func forget(pgid int) {
	reaperMu.Lock()
	defer reaperMu.Unlock()
	if reaperPipe != nil {
		_, _ = fmt.Fprintln(reaperPipe, -pgid)
	}
}

// RunReaper is the reaper: it reads process group numbers from in (a negative number takes one back) until in ends, and then
// kills every group it still holds.
func RunReaper(in io.Reader) {
	groups := map[int]bool{}
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		n, err := strconv.Atoi(scanner.Text())
		switch {
		case err != nil || n == 0:
		case n > 0:
			groups[n] = true
		default:
			delete(groups, -n)
		}
	}
	for pgid := range groups {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
}
