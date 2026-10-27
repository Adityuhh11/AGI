package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWaitForArgReadsExactlyTheFlagAndAProcessId(t *testing.T) {
	for _, tc := range []struct {
		args []string
		pid  int
		ok   bool
	}{
		{[]string{"--wait-for", "4242"}, 4242, true},
		{nil, 0, false},
		{[]string{"--wait-for"}, 0, false},
		{[]string{"--wait-for", "abc"}, 0, false},
		{[]string{"--wait-for", "0"}, 0, false},
		{[]string{"--wait-for", "-5"}, 0, false},
		{[]string{"--wait-for", "7", "extra"}, 0, false},
		{[]string{"--other", "7"}, 0, false},
		{[]string{"7", "--wait-for"}, 0, false},
	} {
		if pid, ok := WaitForArg(tc.args); pid != tc.pid || ok != tc.ok {
			t.Errorf("WaitForArg(%q) = %d, %v; want %d, %v", tc.args, pid, ok, tc.pid, tc.ok)
		}
	}
}

func TestRemoveOldDeletesTheMovedAsideProgramAndIsQuietWhenThereIsNone(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "omni.exe")
	if err := RemoveOld(exe); err != nil {
		t.Fatalf("with nothing to remove: %v", err)
	}
	if err := os.WriteFile(exe, []byte("MZ new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe+".old", []byte("MZ old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RemoveOld(exe); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(exe + ".old"); err == nil {
		t.Error(".old is still there")
	}
	if _, err := os.Stat(exe); err != nil {
		t.Error("the program itself was removed")
	}
}
