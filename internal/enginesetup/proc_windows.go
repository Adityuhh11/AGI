package enginesetup

import (
	"os/exec"
	"strings"
	"syscall"
)

// hide keeps a console window from flashing up for a program the install runs.
func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

// adviceAfterFailure says what to do about a failure of the Python setup that has a known cure: Windows refuses a link in some
// folders (error 448), and another folder usually has none.
func adviceAfterFailure(tail string) string {
	if strings.Contains(tail, "os error 448") {
		return `Windows would not let Python be set up in this folder. Choose another one with Change folder (for example C:\OmniData) and try again`
	}
	return ""
}
