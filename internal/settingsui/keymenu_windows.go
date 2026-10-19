package settingsui

import "github.com/wailsapp/wails/v3/pkg/w32"

// SwallowKeyMenu is the app's WndProcInterceptor. Alt+Space (and a lone Alt) make Windows open the
// window's system menu (restore, move, minimise, close) through a message the web page cannot cancel,
// so recording Alt+Space in the hotkey field would show that menu instead of the key. No window of
// ours needs the menu from the keyboard, so the message is dropped.
func SwallowKeyMenu(_ uintptr, msg uint32, wParam, _ uintptr) (uintptr, bool) {
	// Windows keeps the low four bits of wParam for itself.
	return 0, msg == w32.WM_SYSCOMMAND && wParam&0xFFF0 == w32.SC_KEYMENU
}
