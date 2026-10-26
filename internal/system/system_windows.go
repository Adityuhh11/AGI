package system

import (
	"fmt"
	"os/exec"
	"syscall"
)

var actions = map[string]func() error{}

func init() {
	for id, p := range programs {
		actions[id] = func() error {
			cmd := exec.Command(p.exe, p.args...)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("start %s: %w", p.exe, err)
			}
			return cmd.Process.Release()
		}
	}
	actions["sys:recycle"] = emptyRecycleBin
}

// emptyRecycleBin asks Windows to empty the bin; with no flags it asks the person to confirm first.
func emptyRecycleBin() error {
	proc := syscall.NewLazyDLL("shell32.dll").NewProc("SHEmptyRecycleBinW")
	r, _, _ := proc.Call(0, 0, 0)
	if r != 0 && r != 0x8000FFFF { // 0 is S_OK; the second is what Windows answers when the bin is already empty
		return fmt.Errorf("empty the Recycle Bin: Windows answered %#x", r)
	}
	return nil
}
