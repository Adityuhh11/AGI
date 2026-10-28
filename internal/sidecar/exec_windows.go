//go:build windows

package sidecar

import (
	"context"
	"io"
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// createNoWindow keeps a console window from flashing up for the engine.
const createNoWindow = 0x08000000

type execProcess struct{ cmd *exec.Cmd }

func (p *execProcess) Wait() error { return p.cmd.Wait() }

// Kill ends the process and everything it started. A virtual environment's python.exe is a launcher
// that runs the real interpreter as a child, which in turn runs llama-server: ending only the process we
// started would leave the engine alive on its port, and the next start could not bind it.
func (p *execProcess) Kill() error {
	tree := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(p.cmd.Process.Pid))
	tree.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	_ = tree.Run()
	_ = p.cmd.Process.Kill() // in case taskkill could not run; an error here only says it was already gone
	return nil
}

// ExecStarter runs a program (the engine, or the chat model's server) as a child process with its output sent to output.
func ExecStarter(_ context.Context, spec Spec, output io.Writer) (Process, error) {
	cmd := exec.Command(spec.Exe, spec.Args...)
	cmd.Dir, cmd.Env = spec.Dir, spec.Env
	cmd.Stdout, cmd.Stderr = output, output
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &execProcess{cmd: cmd}, nil
}

// KillWithLauncher puts the calling process in a Job Object that kills everything in it when the
// last handle closes. Children started afterwards (the engine) join the job automatically, so
// they die with the launcher even if it crashes or is killed from the task manager. The job allows breakaway: the
// launcher that replaces this one after an update asks to be created outside it (internal/update), and nothing else does.
func KillWithLauncher() error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK,
		},
	}
	if _, err := windows.SetInformationJobObject(
		job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return err
	}
	// The handle is deliberately never closed: it has to live exactly as long as this process.
	return windows.AssignProcessToJobObject(job, windows.CurrentProcess())
}
