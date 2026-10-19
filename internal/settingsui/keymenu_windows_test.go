package settingsui

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/w32"
)

func TestKeyMenuFilterDropsTheSystemMenuAndReportsAltSpace(t *testing.T) {
	for _, tc := range []struct {
		name        string
		msg         uint32
		wParam      uintptr
		lParam      uintptr
		ctrl, shift bool
		swallowed   bool
		reported    string
	}{
		{"Alt+Space", w32.WM_SYSCOMMAND, w32.SC_KEYMENU, ' ', false, false, true, "Alt+Space"},
		{"the low bits Windows adds", w32.WM_SYSCOMMAND, w32.SC_KEYMENU | 0x3, ' ', false, false, true, "Alt+Space"},
		{"Ctrl+Alt+Shift+Space", w32.WM_SYSCOMMAND, w32.SC_KEYMENU, ' ', true, true, true, "Ctrl+Alt+Shift+Space"},
		{"Alt alone", w32.WM_SYSCOMMAND, w32.SC_KEYMENU, 0, false, false, true, ""},
		{"Alt+F as a menu mnemonic", w32.WM_SYSCOMMAND, w32.SC_KEYMENU, 'f', false, false, true, ""},
		{"closing from the title bar", w32.WM_SYSCOMMAND, w32.SC_CLOSE, ' ', false, false, false, ""},
		{"another message", w32.WM_KEYDOWN, w32.SC_KEYMENU, ' ', false, false, false, ""},
	} {
		var reported string
		filter := keyMenuFilter(func() (bool, bool) { return tc.ctrl, tc.shift }, func(a string) { reported = a })
		if _, got := filter(0, tc.msg, tc.wParam, tc.lParam); got != tc.swallowed {
			t.Errorf("%s: swallowed %v, want %v", tc.name, got, tc.swallowed)
		}
		if reported != tc.reported {
			t.Errorf("%s: reported %q, want %q", tc.name, reported, tc.reported)
		}
	}
}
