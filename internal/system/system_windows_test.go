package system

import "testing"

func TestTheCommandLinesAreWhatWindowsExpects(t *testing.T) {
	if p := programs["sys:lock"]; p.exe != "rundll32.exe" || len(p.args) != 1 || p.args[0] != "user32.dll,LockWorkStation" {
		t.Errorf("lock = %+v", p)
	}
	if p := programs["sys:winsettings"]; p.args[0] != "ms-settings:" {
		t.Errorf("settings = %+v", p)
	}
}
