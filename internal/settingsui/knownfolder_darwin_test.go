package settingsui

import (
	"path/filepath"
	"testing"
)

func TestTheGuideOffersTheFoldersOfAHomeFolderAndCallsVideosMovies(t *testing.T) {
	t.Setenv("HOME", "/Users/me")
	for name, want := range map[string]string{"Documents": "/Users/me/Documents", "Videos": "/Users/me/Movies", "Downloads": "/Users/me/Downloads"} {
		got, err := knownFolder(name)
		if err != nil || filepath.ToSlash(got) != want {
			t.Errorf("knownFolder(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	if _, err := knownFolder("Music"); err == nil {
		t.Error("a folder the guide does not offer must be an error")
	}
}
