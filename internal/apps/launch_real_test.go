package apps

import (
	"context"
	"os"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procFindWindowW  = user32.NewProc("FindWindowW")
	procPostMessageW = user32.NewProc("PostMessageW")
)

func findWindow(class, title string) uintptr {
	var c, t *uint16
	if class != "" {
		c, _ = windows.UTF16PtrFromString(class)
	}
	if title != "" {
		t, _ = windows.UTF16PtrFromString(title)
	}
	h, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(t)))
	return h
}

// TestRealLaunch starts real programs, so it runs only with AGI_REAL_APPS=1 (it opens, then closes, two
// windows), and checks that every kept path-style entry resolves to a file that exists.
func TestRealLaunch(t *testing.T) {
	if os.Getenv("AGI_REAL_APPS") != "1" {
		t.Skip("set AGI_REAL_APPS=1 to start real programs")
	}
	data, err := startApps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	kept, err := parse(data)
	if err != nil {
		t.Fatal(err)
	}
	var withFile, missing int
	for _, app := range kept {
		if file, ok := Path(app); ok {
			withFile++
			if _, err := os.Stat(file); err != nil {
				missing++
				t.Logf("resolves to a file that is not there: %s -> %s", app.Name, file)
			}
		}
	}
	t.Logf("%d of %d kept entries name a file; %d of those are missing", withFile, len(kept), missing)

	tests := []struct {
		name, id, class, title string
	}{
		{"7-Zip (a path under a known folder)", `{6D809377-6AF0-444B-8957-A3773F02200E}\7-Zip\7zFM.exe`, "7-Zip::FM", ""},
		{"Control Panel (a plain id)", `Microsoft.Windows.ControlPanel`, "CabinetWClass", "Control Panel"},
	}
	for _, tt := range tests {
		if _, ok := indexHas(kept, tt.id); !ok {
			t.Logf("%s: not installed here, skipped", tt.name)
			continue
		}
		start := time.Now()
		if err := Launch(App{ID: tt.id, Name: tt.name}); err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		var h uintptr
		for time.Since(start) < 8*time.Second {
			if h = findWindow(tt.class, tt.title); h != 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if h == 0 {
			t.Errorf("%s: no window within 8 s", tt.name)
			continue
		}
		t.Logf("%s: window after %v", tt.name, time.Since(start).Round(time.Millisecond))
		procPostMessageW.Call(h, 0x0010, 0, 0) // WM_CLOSE
		time.Sleep(500 * time.Millisecond)
	}
}

func indexHas(apps []App, id string) (App, bool) {
	for _, a := range apps {
		if a.ID == id {
			return a, true
		}
	}
	return App{}, false
}

// TestRealFocus brings an app that is already open to the front, so it runs only with AGI_REAL_FOCUS=<name>
// (the Start Menu name of a running app, such as Zed) and moves the user's windows.
func TestRealFocus(t *testing.T) {
	name := os.Getenv("AGI_REAL_FOCUS")
	if name == "" {
		t.Skip("set AGI_REAL_FOCUS=<name of a running app> to focus a real window")
	}
	start := time.Now()
	if !focusRunning(App{Name: name}, "") {
		t.Fatalf("no window of %q was found and focused", name)
	}
	t.Logf("%s focused in %v", name, time.Since(start).Round(time.Microsecond))
}
