package update

import (
	"errors"
	"syscall"
	"time"
)

// StartNew is not used on a Mac yet: nothing installs a new program (asset_darwin.go).
func StartNew(string, int) error { return errors.New("the one-click update is not available on macOS") }

// WaitForExit waits until process pid has ended, at most timeout, and says whether it has. A process that no longer exists
// counts as ended. (kill with signal 0 only asks whether the process is there.)
func WaitForExit(pid int, timeout time.Duration) bool {
	for deadline := time.Now().Add(timeout); ; time.Sleep(50 * time.Millisecond) {
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
	}
}
