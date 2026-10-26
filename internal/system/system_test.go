package system

import "testing"

func TestEveryOfferedActionIsKnownAndAnUnknownOneIsRefused(t *testing.T) {
	for _, id := range []string{"sys:lock", "sys:sleep", "sys:taskmgr", "sys:winsettings", "sys:recycle"} {
		if _, ok := actions[id]; !ok {
			t.Errorf("%s has no action", id)
		}
	}
	if err := Run("sys:shutdown"); err == nil {
		t.Error("shutting down must not be offered")
	}
}

func TestTheCommandLinesAreWhatWindowsExpects(t *testing.T) {
	if p := programs["sys:lock"]; p.exe != "rundll32.exe" || len(p.args) != 1 || p.args[0] != "user32.dll,LockWorkStation" {
		t.Errorf("lock = %+v", p)
	}
	if p := programs["sys:winsettings"]; p.args[0] != "ms-settings:" {
		t.Errorf("settings = %+v", p)
	}
}
