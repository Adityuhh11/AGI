package testpath

import (
	"runtime"
	"testing"
)

func TestPKeepsWindowsPathsOnWindowsAndMakesThemPOSIXElsewhere(t *testing.T) {
	for in, mac := range map[string]string{
		`C:\Users\me\Docs`:   "/C/Users/me/Docs",
		`D:\Files\`:          "/D/Files/",
		`C:\a`:               "/C/a",
		`relative\Omni`:      "relative/Omni",
		`C:\Users\me\` + "x": "/C/Users/me/x",
	} {
		want := mac
		if runtime.GOOS == "windows" {
			want = in
		}
		if got := P(in); got != want {
			t.Errorf("P(%q) = %q, want %q", in, got, want)
		}
	}
}
