package settingsui

import (
	"errors"
	"testing"
)

func TestMovedFindsAUserFolderWindowsKeepsElsewhere(t *testing.T) {
	home := `C:\Users\me`
	lookup := func(name string) (string, error) {
		switch name {
		case "Documents":
			return `C:\Users\me\OneDrive\Documents`, nil
		case "Pictures":
			return `C:\Users\me\Pictures`, nil
		}
		return "", errors.New("unknown")
	}
	tests := []struct{ root, want string }{
		{`C:\Users\me\Documents`, `C:\Users\me\OneDrive\Documents`},
		{`c:\users\ME\documents\`, `C:\Users\me\OneDrive\Documents`},
		{`C:\Users\me\Pictures`, ""},       // not moved
		{`C:\Users\me\Desktop`, ""},        // the lookup failed: say nothing
		{`C:\Users\me\Documents\Work`, ""}, // a folder inside it, not the user folder itself
		{`D:\Projects`, ""},
	}
	for _, tt := range tests {
		if got := moved(tt.root, home, lookup); got != tt.want {
			t.Errorf("moved(%q) = %q, want %q", tt.root, got, tt.want)
		}
	}
}
