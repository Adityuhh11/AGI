package enginesetup

import "os/exec"

// hide has nothing to do on a Mac: a program started without a terminal opens no window.
func hide(*exec.Cmd) {}
