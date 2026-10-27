// Package system runs the few Windows actions the bar offers beside launching programs: lock, sleep, the Task Manager, the
// Recycle Bin and Windows Settings. Shutting down and restarting are left out on purpose: one mistyped Enter should not do that.
package system

import "fmt"

// Run does the action with this id ("sys:lock").
func Run(id string) error {
	if run, ok := actions[id]; ok {
		return run()
	}
	return fmt.Errorf("unknown system action %q", id)
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
