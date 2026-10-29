package system

import (
	"fmt"
	"os"
	"os/exec"
)

// Commands are the rows the bar offers on a Mac. Shut down and restart are left out, as on Windows.
var Commands = []Command{
	{"sys:lock", "Lock the screen"},
	{"sys:sleep", "Sleep"},
	{"sys:activitymonitor", "Activity Monitor"},
	{"sys:recycle", "Empty the Trash"},
	{"sys:systemsettings", "System Settings"},
}

// cgSession is the helper that locks the session without asking for any permission. Apple has moved it before, so it is used when it is
// there; otherwise the display is put to sleep, which locks the Mac when it is set to ask for the password at once (the macOS default).
// (ponytail: verify on the macOS versions in use; if neither locks reliably the row should go.)
const cgSession = "/System/Library/CoreServices/Menu Extras/User.menu/Contents/Resources/CGSession"

func lock() error {
	if _, err := os.Stat(cgSession); err == nil {
		return run(cgSession, "-suspend")
	}
	return run("pmset", "displaysleepnow")
}

func init() {
	actions["sys:lock"] = lock
	actions["sys:sleep"] = func() error { return run("pmset", "sleepnow") }
	actions["sys:activitymonitor"] = func() error { return run("open", "-a", "Activity Monitor") }
	actions["sys:systemsettings"] = func() error { return run("open", "x-apple.systempreferences:") }
	// Finder asks the person to confirm, as the Recycle Bin does on Windows. The first time, macOS asks whether Omni may control Finder.
	actions["sys:recycle"] = func() error {
		return run("osascript", "-e", `tell application "Finder" to empty the trash`)
	}
}

// Beep plays the system's notification sound (a finished timer). It is silent on a Mac whose sound is muted, which is right.
func Beep() {
	_ = exec.Command("afplay", "/System/Library/Sounds/Glass.aiff").Start()
}

// run starts a program and lets it go.
func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", name, err)
	}
	return cmd.Process.Release()
}
