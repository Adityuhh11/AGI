package enginesetup

import (
	"os/exec"
	"syscall"
)

// hide keeps a console window from flashing up for a program the install runs.
func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
