// Package system runs the few actions on the computer the bar offers beside launching programs: lock, sleep, the process monitor, the
// bin and the system settings. Shutting down and restarting are left out on purpose: one mistyped Enter should not do that.
//
// What each system can do, and what it calls it, is in system_windows.go and system_darwin.go.
package system

import "fmt"

// Command is an action the bar offers as a row: its id ("sys:lock", which the bar's history keeps) and what the row says.
type Command struct{ ID, Title string }

// Run does the action with this id.
func Run(id string) error {
	if run, ok := actions[id]; ok {
		return run()
	}
	return fmt.Errorf("unknown system action %q", id)
}

// actions are the things Run can do, filled in by each system's file.
var actions = map[string]func() error{}
