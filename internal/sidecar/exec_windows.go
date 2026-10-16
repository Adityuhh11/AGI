//go:build windows

package sidecar

import (
	"context"
	"io"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// createNoWindow keeps a console window from flashing up for the engine.
const createNoWindow = 0x08000000

type execProcess struct{ cmd *exec.Cmd }

func (p *execProcess) Wait() error { return p.cmd.Wait() }
func (p *execProcess) Kill() error { return p.cmd.Process.Kill() }

// execStarter runs the engine as a child process with its output sent to output.
func execStarter(_ context.Context, spec Spec, output io.Writer) (Process, error) {
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
// they die with the launcher even if it crashes or is killed from the task manager.
func KillWithLauncher() error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
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
