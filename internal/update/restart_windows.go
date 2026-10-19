package update

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// StartNew starts the program at exe as the new launcher, told to wait for process pid (the one calling) to end. The
// launcher runs inside a job that kills everything in it when the launcher ends (that is how the search engine dies with
// it), so the new one must be created outside that job or it would die with the old one: the job allows it, and this asks for it.
func StartNew(exe string, pid int) error {
	cmd := exec.Command(exe, WaitForFlag, strconv.Itoa(pid))
	cmd.Dir = filepath.Dir(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_BREAKAWAY_FROM_JOB | windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start the new program: %w", err)
	}
	return cmd.Process.Release()
}

// WaitForExit waits until process pid has ended, at most timeout, and says whether it has. A process that no longer
// exists counts as ended.
func WaitForExit(pid int, timeout time.Duration) bool {
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return true // nothing to open: it is gone (or cannot be waited for, which this cannot tell from gone)
	}
	defer windows.CloseHandle(process)
	event, err := windows.WaitForSingleObject(process, uint32(timeout.Milliseconds()))
	return err == nil && event == windows.WAIT_OBJECT_0
}
