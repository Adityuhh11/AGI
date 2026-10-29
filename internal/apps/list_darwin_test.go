package apps

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestScanFindsBundlesDirectlyAndOneFolderDeepAndNamesThemAsFinderDoes(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{
		"Safari.app", "Visual Studio Code.app", "Adobe Photoshop 2025/Adobe Photoshop 2025.app", "Adobe Photoshop 2025/Plug-ins",
		"Deep/Deeper/Hidden.app", ".Trashes/Gone.app", "Utilities/Terminal.app",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got := scan([]string{root, filepath.Join(root, "nowhere")})
	have := map[string]string{}
	for _, app := range got {
		have[app.Name] = app.ID
	}
	for _, name := range []string{"Safari", "Visual Studio Code", "Adobe Photoshop 2025", "Terminal"} {
		if have[name] == "" {
			t.Errorf("%s was not found in %v", name, names(got))
		}
	}
	for _, name := range []string{"Hidden", "Gone", "notes.txt"} {
		if have[name] != "" {
			t.Errorf("%s must not be listed (too deep, hidden or not an app)", name)
		}
	}
	if have["Safari"] != filepath.Join(root, "Safari.app") {
		t.Errorf("the id is the bundle path, got %q", have["Safari"])
	}
}

func TestScanListsAnExtraBundleOnceAndOnlyWhenItExists(t *testing.T) {
	root := t.TempDir()
	finder := filepath.Join(root, "Finder.app")
	if err := os.MkdirAll(finder, 0o755); err != nil {
		t.Fatal(err)
	}
	got := scan([]string{root}, finder, filepath.Join(root, "Missing.app"))
	if len(got) != 1 || got[0].Name != "Finder" {
		t.Fatalf("scan = %v, want Finder once", got)
	}
}

func TestLaunchRefusesABundleThatIsGone(t *testing.T) {
	err := Launch(App{ID: filepath.Join(t.TempDir(), "Gone.app"), Name: "Gone"})
	if err == nil || !errors.Is(err, ErrGone) {
		t.Fatalf("Launch = %v, want ErrGone", err)
	}
}

func TestWithoutDeadKeepsOnlyBundlesThatExist(t *testing.T) {
	root := t.TempDir()
	alive := filepath.Join(root, "Alive.app")
	if err := os.MkdirAll(alive, 0o755); err != nil {
		t.Fatal(err)
	}
	got := withoutDead([]App{{ID: alive, Name: "Alive"}, {ID: filepath.Join(root, "Dead.app"), Name: "Dead"}})
	if len(got) != 1 || got[0].Name != "Alive" {
		t.Fatalf("withoutDead = %v", got)
	}
}
