package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// swap puts the new program where the running one is. Windows lets a running program be renamed but not deleted or
// overwritten, so the running one becomes agi.exe.old and the new one takes its name. A rename is retried a few times
// (a virus scanner may be holding a file it has just seen written). If the second rename fails the first is undone, so
// there is always a program at exePath.
func (i *Installer) swap(exePath, newPath string) error {
	old := exePath + ".old"
	if err := os.Remove(old); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear the earlier %s (is another copy of the program still running?): %w", filepath.Base(old), err)
	}
	if err := i.retry(func() error { return i.rename(exePath, old) }); err != nil {
		return fmt.Errorf("move the running program aside: %w", err)
	}
	if err := i.retry(func() error { return i.rename(newPath, exePath) }); err != nil {
		if back := i.retry(func() error { return i.rename(old, exePath) }); back != nil {
			return fmt.Errorf("install the new program: %v; and the old one could not be put back: %v (it is %s)", err, back, old)
		}
		return fmt.Errorf("install the new program (the old one is back in place): %w", err)
	}
	return nil
}

// retry runs f up to i.attempts times, pausing between tries, and returns the last error.
func (i *Installer) retry(f func() error) (err error) {
	for n := 0; n < i.attempts; n++ {
		if err = f(); err == nil {
			return nil
		}
		if n+1 < i.attempts {
			i.sleep(i.pause)
		}
	}
	return err
}

// Real values; the tests replace them.
const (
	swapAttempts = 5
	swapPause    = 200 * time.Millisecond
)
