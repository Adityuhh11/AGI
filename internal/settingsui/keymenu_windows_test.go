package settingsui

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/w32"
)

func TestSwallowKeyMenuDropsOnlyTheKeyboardSystemMenu(t *testing.T) {
	for _, tc := range []struct {
		name   string
		msg    uint32
		wParam uintptr
		want   bool
	}{
		{"Alt+Space", w32.WM_SYSCOMMAND, w32.SC_KEYMENU, true},
		{"the low bits Windows adds", w32.WM_SYSCOMMAND, w32.SC_KEYMENU | 0x3, true},
		{"closing from the title bar", w32.WM_SYSCOMMAND, w32.SC_CLOSE, false},
		{"another message", w32.WM_KEYDOWN, w32.SC_KEYMENU, false},
	} {
		if _, got := SwallowKeyMenu(0, tc.msg, tc.wParam, 0); got != tc.want {
			t.Errorf("%s: swallowed %v, want %v", tc.name, got, tc.want)
		}
	}
}
