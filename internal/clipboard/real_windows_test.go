package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// These tests use the real clipboard and open a window, so they run only with OMNI_REAL_CLIPBOARD=1. They
// save the text that was on the clipboard and put it back.

var (
	procFindWindowW = user32.NewProc("FindWindowW")
	procSetFg       = user32.NewProc("SetForegroundWindow")
	procKeybdEvent  = user32.NewProc("keybd_event")
)

func realOnly(t *testing.T) {
	t.Helper()
	if os.Getenv("OMNI_REAL_CLIPBOARD") != "1" {
		t.Skip("set OMNI_REAL_CLIPBOARD=1 to use the real clipboard and open a window")
	}
	saved := powershell(t, "Get-Clipboard -Raw")
	t.Cleanup(func() {
		powershell(t, "Set-Clipboard -Value $env:OMNI_SAVED", "OMNI_SAVED="+strings.TrimRight(saved, "\r\n"))
	})
}

func powershell(t *testing.T, script string, env ...string) string {
	t.Helper()
	cmd := exec.Command("powershell.exe", "-NoProfile", "-STA", "-Command", script)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("powershell %q: %v\n%s", script, err, out)
	}
	return string(out)
}

func findWindow(class, title string) uintptr {
	var c, ti *uint16
	if class != "" {
		c, _ = windows.UTF16PtrFromString(class)
	}
	if title != "" {
		ti, _ = windows.UTF16PtrFromString(title)
	}
	h, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(ti)))
	return h
}

// toForeground makes a window the foreground window from a program that has no window of its own: a tap on Alt
// allows it, the way the bar's own handover is allowed because the bar had the keyboard.
func toForeground(hwnd uintptr) {
	procKeybdEvent.Call(0x12, 0, 0, 0)
	procKeybdEvent.Call(0x12, 0, 2, 0)
	procSetFg.Call(hwnd)
}

// A marked copy is what a password manager makes: the text, and a format that says "keep this out of any history".
func TestRealReadHonoursTheFormatsPasswordManagersSet(t *testing.T) {
	realOnly(t)
	mark := func(format string, value ...byte) string {
		bytes := make([]string, len(value))
		for i, b := range value {
			bytes[i] = fmt.Sprint(b)
		}
		return `Add-Type -AssemblyName System.Windows.Forms;
$d = New-Object System.Windows.Forms.DataObject; $d.SetText('marked text');
$d.SetData('` + format + `', (New-Object System.IO.MemoryStream (,[byte[]](` + strings.Join(bytes, ",") + `))));
[System.Windows.Forms.Clipboard]::SetDataObject($d, $true)`
	}
	for _, tc := range []struct {
		name    string
		script  string
		private bool
	}{
		{"plain text", "Set-Clipboard -Value 'marked text'", false},
		{"the exclude marker", mark(formatExclude, 1, 0, 0, 0), true},
		{"CanIncludeInClipboardHistory = 0", mark(formatHistory, 0, 0, 0, 0), true},
		{"CanIncludeInClipboardHistory = 1", mark(formatHistory, 1, 0, 0, 0), false},
		{"CanUploadToCloudClipboard = 0", mark(formatCloud, 0, 0, 0, 0), true},
	} {
		powershell(t, tc.script)
		clip, err := WindowsSource{}.Read()
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if clip.Text != "marked text" || clip.Private != tc.private {
			t.Errorf("%s: read %+v, want the text with private=%v", tc.name, clip, tc.private)
		}
	}
}

func TestRealSequenceMovesWhenSomethingIsCopied(t *testing.T) {
	realOnly(t)
	before := WindowsSource{}.Sequence()
	powershell(t, "Set-Clipboard -Value 'x1'")
	if after := (WindowsSource{}).Sequence(); after == before {
		t.Fatal("the counter did not move")
	}
}

const targetScript = `
param($out, $done)
Add-Type -AssemblyName System.Windows.Forms
$f = New-Object Windows.Forms.Form; $f.Text = 'omni-paste-target'; $f.Width = 500; $f.Height = 240; $f.StartPosition = 'CenterScreen'
$t = New-Object Windows.Forms.TextBox; $t.Multiline = $true; $t.Dock = 'Fill'; $f.Controls.Add($t)
$timer = New-Object Windows.Forms.Timer; $timer.Interval = 40
$timer.Add_Tick({ [IO.File]::WriteAllText($out, $t.Text); if (Test-Path $done) { $f.Close() } })
$timer.Start()
$f.Add_Shown({ $f.Activate(); $t.Focus() })
[Windows.Forms.Application]::Run($f)
`

// TestRealPasteIntoAnotherProgram is the spike of docs/eval/2026-10-19-paste.md: it hands the keyboard to a
// real window from another program the way the bar does, and types Ctrl+V after different waits.
func TestRealPasteIntoAnotherProgram(t *testing.T) {
	realOnly(t)
	dir := t.TempDir()
	script, out, done := filepath.Join(dir, "target.ps1"), filepath.Join(dir, "out.txt"), filepath.Join(dir, "done")
	if err := os.WriteFile(script, []byte(targetScript), 0o644); err != nil {
		t.Fatal(err)
	}
	target := exec.Command("powershell.exe", "-NoProfile", "-STA", "-File", script, out, done)
	if err := target.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.WriteFile(done, nil, 0o644)
		time.Sleep(300 * time.Millisecond)
		_ = target.Process.Kill()
	})
	var window uintptr
	for deadline := time.Now().Add(15 * time.Second); window == 0 && time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		window = findWindow("", "omni-paste-target")
	}
	if window == 0 {
		t.Fatal("the target window never appeared")
	}
	tray := findWindow("Shell_TrayWnd", "")
	if tray == 0 {
		t.Fatal("no taskbar to hand the keyboard away from")
	}

	for _, settleFor := range []time.Duration{0, 10 * time.Millisecond, settle} {
		pasted, took := 0, time.Duration(0)
		const trials = 8
		for i := range trials {
			token := fmt.Sprintf("tok-%dms-%d;", settleFor.Milliseconds(), i)
			toForeground(tray) // the program the keyboard goes back to is not in front: the bar is
			time.Sleep(150 * time.Millisecond)
			paster := NewPaster(
				func(text string) bool {
					powershell(t, "Set-Clipboard -Value $env:OMNI_TEXT", "OMNI_TEXT="+text)
					return true
				},
				func() uintptr { return window },
				func() { toForeground(window) },
			)
			paster.Sleep = func(d time.Duration) {
				if d == settle {
					d = settleFor
				}
				time.Sleep(d)
			}
			start := time.Now()
			if err := paster.Paste(token); err != nil {
				t.Fatalf("settle %v, trial %d: %v", settleFor, i, err)
			}
			took += time.Since(start)
			for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(40 * time.Millisecond) {
				if raw, _ := os.ReadFile(out); strings.Contains(string(raw), token) {
					pasted++
					break
				}
			}
		}
		t.Logf("settle %-5v: %d of %d pasted", settleFor, pasted, trials)
		if settleFor == settle && pasted != trials {
			t.Errorf("with the settle time the product uses (%v) only %d of %d pastes arrived", settle, pasted, trials)
		}
	}
}
