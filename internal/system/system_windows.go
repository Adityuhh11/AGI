package system

import (
	"fmt"
	"os/exec"
	"syscall"
)

// Commands are the rows the bar offers on Windows.
var Commands = []Command{
	{"sys:lock", "Lock the computer"},
	{"sys:sleep", "Sleep"},
	{"sys:taskmgr", "Task Manager"},
	{"sys:recycle", "Empty the Recycle Bin"},
	{"sys:winsettings", "Windows Settings"},
}

// program is an action that starts a program with arguments.
type program struct {
	exe  string
	args []string
}

// programs are the actions that are just a command line. Sleep goes through powrprof; Windows hibernates instead when
// hibernation is switched on and sleep is not available, which is the computer's own choice.
var programs = map[string]program{
	"sys:lock":        {"rundll32.exe", []string{"user32.dll,LockWorkStation"}},
	"sys:sleep":       {"rundll32.exe", []string{"powrprof.dll,SetSuspendState", "0,1,0"}},
	"sys:taskmgr":     {"taskmgr.exe", nil},
	"sys:winsettings": {"explorer.exe", []string{"ms-settings:"}},
}

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

// Beep plays the system's notification sound (a finished timer). It makes no sound on a computer whose sounds are off, which is right.
func Beep() {
	const iconAsterisk = 0x40
	_, _, _ = syscall.NewLazyDLL("user32.dll").NewProc("MessageBeep").Call(iconAsterisk)
}
