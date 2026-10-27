package autostart

import (
	"fmt"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

// scratch points the package at keys of its own, so the test never touches the real Run key.
func scratch(t *testing.T) {
	t.Helper()
	root := fmt.Sprintf(`Software\Omni-autostart-test-%d`, time.Now().UnixNano())
	oldRun, oldApproved := runKey, approvedKey
	runKey, approvedKey = root+`\Run`, root+`\Approved`
	t.Cleanup(func() {
		runKey, approvedKey = oldRun, oldApproved
		_ = registry.DeleteKey(registry.CURRENT_USER, root+`\Run`)
		_ = registry.DeleteKey(registry.CURRENT_USER, root+`\Approved`)
		_ = registry.DeleteKey(registry.CURRENT_USER, root)
	})
}

func TestSetAndEnabled(t *testing.T) {
	scratch(t)
	if Enabled() {
		t.Fatal("enabled before anything was written")
	}
	if err := Set(true); err != nil {
		t.Fatal(err)
	}
	if !Enabled() {
		t.Fatal("not enabled after Set(true)")
	}

	// Task Manager switched the entry off: it is off, until the person turns it on here.
	k, _, err := registry.CreateKey(registry.CURRENT_USER, approvedKey, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	if err := k.SetBinaryValue(valueName, []byte{3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	k.Close()
	if Enabled() {
		t.Fatal("enabled although Task Manager switched it off")
	}
	if err := Set(true); err != nil {
		t.Fatal(err)
	}
	if !Enabled() {
		t.Fatal("Set(true) did not clear the Task Manager flag")
	}

	if err := Set(false); err != nil {
		t.Fatal(err)
	}
	if Enabled() {
		t.Fatal("still enabled after Set(false)")
	}
	if err := Set(false); err != nil {
		t.Fatalf("turning off twice: %v", err)
	}
}

func TestAnEntryForAnotherPathIsOff(t *testing.T) {
	scratch(t)
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	if err := k.SetStringValue(valueName, `"C:\somewhere\else\omni.exe"`); err != nil {
		t.Fatal(err)
	}
	if Enabled() {
		t.Fatal("an entry that starts another copy counted as enabled")
	}
}
