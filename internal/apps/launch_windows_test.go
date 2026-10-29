package apps

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveHandlesEveryWayWindowsNamesAnEntry(t *testing.T) {
	old := knownFolder
	knownFolder = func(id string) (string, error) {
		if id == "{PF}" {
			return `C:\Program Files`, nil
		}
		return "", errors.New("unknown folder")
	}
	defer func() { knownFolder = old }()

	tests := []struct {
		id        string
		run, file string
	}{
		{`steam://rungameid/1174180`, `steam://rungameid/1174180`, ""},
		{`{PF}\7-Zip\7zFM.exe`, `shell:AppsFolder\{PF}\7-Zip\7zFM.exe`, `C:\Program Files\7-Zip\7zFM.exe`},
		{`{NOPE}\Thing\thing.exe`, `shell:AppsFolder\{NOPE}\Thing\thing.exe`, ""},
		{`C:\Python313\python.exe`, `C:\Python313\python.exe`, `C:\Python313\python.exe`},
		{`Microsoft.WindowsCalculator_8wekyb3d8bbwe!App`, `shell:AppsFolder\Microsoft.WindowsCalculator_8wekyb3d8bbwe!App`, ""},
		{`Chrome`, `shell:AppsFolder\Chrome`, ""},
	}
	for _, tt := range tests {
		if got := resolve(tt.id); got.run != tt.run || got.file != tt.file {
			t.Errorf("resolve(%q) = {run %q, file %q}, want {run %q, file %q}", tt.id, got.run, got.file, tt.run, tt.file)
		}
	}
}

func TestLaunchRefusesAnAppWhoseFileIsGoneWithoutAskingTheShell(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "uninstalled.exe") // an absolute path that does not exist
	err := Launch(App{ID: gone, Name: "Uninstalled"})
	if !errors.Is(err, ErrGone) {
		t.Fatalf("Launch = %v, want ErrGone", err)
	}
	if err := RunAsAdministrator(App{ID: gone, Name: "Uninstalled"}); !errors.Is(err, ErrGone) {
		t.Fatalf("RunAsAdministrator = %v, want ErrGone", err)
	}
}

func TestWithoutDeadDropsOnlyEntriesWhoseFileIsGone(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "present.exe")
	if err := os.WriteFile(present, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	apps := []App{
		{ID: present, Name: "Present"},
		{ID: filepath.Join(dir, "gone.exe"), Name: "Gone"},
		{ID: `Microsoft.WindowsCalculator_8wekyb3d8bbwe!App`, Name: "Calculator"}, // names no file: kept
	}
	if got := names(withoutDead(apps)); got != "Present|Calculator" {
		t.Fatalf("kept %s", got)
	}
}

func TestPathAndElevationNeedAFile(t *testing.T) {
	store := App{ID: `Microsoft.WindowsCalculator_8wekyb3d8bbwe!App`, Name: "Calculator"}
	if _, ok := Path(store); ok {
		t.Fatal("a Store app does not say which file it starts")
	}
	if err := RunAsAdministrator(store); err == nil {
		t.Fatal("a Store app cannot be elevated from a path it does not have")
	}
	if err := RevealInFileManager(store); err == nil {
		t.Fatal("there is no file to show for a Store app")
	}
	file := filepath.Join(t.TempDir(), "tool.exe")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := Path(App{ID: file}); !ok || got != file {
		t.Fatalf("Path = %q, %v", got, ok)
	}
}

func TestSameAppMatchesByFileWhenThereIsOneAndByNameWhenNot(t *testing.T) {
	describe := func(exe string) string {
		if exe == `C:\Program Files\Google\Chrome\Application\chrome.exe` {
			return "Google Chrome"
		}
		return ""
	}
	tests := []struct {
		name, file, exe string
		want            bool
	}{
		{"Zed", "", `C:\Users\me\AppData\Local\Programs\Zed\Zed.exe`, true},
		{"Zen", "", `C:\Program Files\Zen Browser\zen.exe`, true},
		{"Google Chrome", "", `C:\Program Files\Google\Chrome\Application\chrome.exe`, true},
		{"Zen Private Browsing", "", `C:\Program Files\Zen Browser\zen.exe`, false},
		{"Notepad", "", `C:\Windows\explorer.exe`, false},
		{"7-Zip File Manager", `C:\Program Files\7-Zip\7zFM.exe`, `c:\program files\7-zip\7zfm.exe`, true},
		{"7-Zip File Manager", `C:\Program Files\7-Zip\7zFM.exe`, `C:\Program Files\7-Zip\7zG.exe`, false},
	}
	for _, tt := range tests {
		if got := sameApp(tt.name, tt.file, tt.exe, describe); got != tt.want {
			t.Errorf("sameApp(%q, %q, %q) = %v, want %v", tt.name, tt.file, tt.exe, got, tt.want)
		}
	}
}
