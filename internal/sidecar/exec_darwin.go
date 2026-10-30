package sidecar

import (
	"context"
	"io"
	"os/exec"
	"syscall"
)

type execProcess struct {
	cmd  *exec.Cmd
	pgid int
}

// Wait returns when the process has ended. What it started and left running goes with it: the engine ending by itself would
// otherwise leave its llama-server behind, holding the model's memory, with nobody to end it.
func (p *execProcess) Wait() error {
	err := p.cmd.Wait()
	_ = p.Kill() // nothing left in the group is the usual case, and an error then only says so
	forget(p.pgid)
	return err
}

// Kill ends the process and everything it started. The engine's interpreter starts llama-server, so ending only the process we
// started would leave the server alive on its port and holding its memory: the whole process group goes, as taskkill /T does
// on Windows. (ponytail: SIGKILL, no grace period; the index is crash-safe, as it has to be on Windows where the end is as abrupt.)
func (p *execProcess) Kill() error {
	if err := syscall.Kill(-p.pgid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}

// ExecStarter runs a program (the engine, or the chat model's server) as a child process with its output sent to output, in
// a process group of its own that the reaper knows (KillWithLauncher).
func ExecStarter(_ context.Context, spec Spec, output io.Writer) (Process, error) {
	cmd := exec.Command(spec.Exe, spec.Args...)
	cmd.Dir, cmd.Env = spec.Dir, spec.Env
	cmd.Stdout, cmd.Stderr = output, output
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // Pgid 0: the group's number is the process's own
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	watch(cmd.Process.Pid)
	return &execProcess{cmd: cmd, pgid: cmd.Process.Pid}, nil
}
