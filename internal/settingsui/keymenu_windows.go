package settingsui

import "github.com/wailsapp/wails/v3/pkg/w32"

// KeyMenuFilter returns the app's WndProcInterceptor. Windows (and Chromium under the web view) treat
// Alt+Space and a lone Alt as the key that opens the window's system menu (restore, move, minimise,
// close): the page never gets a keydown for Alt+Space, and the menu would open over the settings. No
// window of ours wants that menu from the keyboard, so the message is dropped, and Alt+Space, the one
// combination a launcher user is likely to want, is reported to onAltSpace as an accelerator so the
// hotkey field can record it.
func KeyMenuFilter(onAltSpace func(accelerator string)) func(hwnd uintptr, msg uint32, wParam, lParam uintptr) (uintptr, bool) {
	return keyMenuFilter(heldModifiers, onAltSpace)
}

func keyMenuFilter(held func() (ctrl, shift bool), onAltSpace func(string)) func(uintptr, uint32, uintptr, uintptr) (uintptr, bool) {
	return func(_ uintptr, msg uint32, wParam, lParam uintptr) (uintptr, bool) {
		// Windows keeps the low four bits of wParam for itself.
		if msg != w32.WM_SYSCOMMAND || wParam&0xFFF0 != w32.SC_KEYMENU {
			return 0, false
		}
		if lParam == ' ' { // the character typed with Alt; 0 means Alt alone or F10
			ctrl, shift := held()
			onAltSpace(altSpace(ctrl, shift))
		}
		return 0, true
	}
}

// altSpace names the combination in the order the hotkey field uses (Ctrl, Alt, Shift, key).
func altSpace(ctrl, shift bool) string {
	name := "Alt+Space"
	if shift {
		name = "Alt+Shift+Space"
	}
	if ctrl {
		name = "Ctrl+" + name
	}
	return name
}

func heldModifiers() (ctrl, shift bool) {
	return w32.GetKeyState(w32.VK_CONTROL) < 0, w32.GetKeyState(w32.VK_SHIFT) < 0
}
