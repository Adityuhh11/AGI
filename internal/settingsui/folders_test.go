package settingsui

import (
	"errors"
	"strings"
	"testing"

	"omni/internal/testpath"
)

func TestSuggestedFoldersStartTickedOnlyOnAFirstRunAndFollowTheRootsAfter(t *testing.T) {
	known := func(name string) (string, error) {
		switch name {
		case "Documents":
			return testpath.P(`C:\Users\me\OneDrive\Documents`), nil // moved by OneDrive
		case "Pictures", "Downloads", "Desktop":
			return testpath.P(`C:\Users\me\`) + name, nil
		}
		return "", errors.New("no such folder") // Videos
	}
	isDir := func(path string) bool { return !strings.HasSuffix(path, "Desktop") } // Desktop is not there
	describe := func(roots []string) string {
		var out []string
		for _, f := range suggestFolders(known, isDir, roots) {
			mark := "-"
			if f.Checked {
				mark = "x"
			}
			out = append(out, mark+f.Name+"="+f.Path)
		}
		return strings.Join(out, " | ")
	}
	want := `xDocuments=C:\Users\me\OneDrive\Documents | xPictures=C:\Users\me\Pictures | -Downloads=C:\Users\me\Downloads`
	if got := describe(nil); got != want {
		t.Errorf("first run:\n got %s\nwant %s", got, want)
	}
	want = `-Documents=C:\Users\me\OneDrive\Documents | -Pictures=C:\Users\me\Pictures | xDownloads=C:\Users\me\Downloads | xdev=D:\dev`
	if got := describe([]string{testpath.P(`c:\users\ME\downloads`), testpath.P(`D:\dev`)}); got != want {
		t.Errorf("with roots:\n got %s\nwant %s", got, want)
	}
}

func TestSetRootsReplacesTheFoldersAndForgetsWhatWasLeftOutOfARemovedOne(t *testing.T) {
	h := newHarness(t)
	if msg := h.svc.SetRoots([]string{testpath.P(`C:\a`), testpath.P(`C:\b`)}); msg != "" {
		t.Fatal(msg)
	}
	h.picked = testpath.P(`C:\a\big`)
	if msg := h.svc.LeaveOut(testpath.P(`C:\a`)); msg != "" {
		t.Fatal(msg)
	}
	if got := h.store.Get().Exclude; len(got) != 1 || got[0] != testpath.P(`C:\a\big`) {
		t.Fatalf("exclude = %v", got)
	}
	if msg := h.svc.SetRoots([]string{testpath.P(`C:\b`)}); msg != "" {
		t.Fatal(msg)
	}
	if got := h.store.Get(); len(got.Roots) != 1 || len(got.Exclude) != 0 {
		t.Fatalf("roots %v, exclude %v: a folder left out inside a removed one must go with it", got.Roots, got.Exclude)
	}
	if msg := h.svc.SetRoots([]string{`relative`}); msg == "" {
		t.Fatal("a relative folder must be refused")
	}
}

func TestLeaveOutOnlyTakesAFolderInsideTheRootAndStopLeavingOutPutsItBack(t *testing.T) {
	h := newHarness(t)
	_ = h.svc.SetRoots([]string{testpath.P(`C:\a`)})
	for picked, wantRefused := range map[string]bool{testpath.P(`C:\elsewhere`): true, testpath.P(`C:\a`): true, "": false, testpath.P(`C:\a\node_modules`): false} {
		h.picked = picked
		if msg := h.svc.LeaveOut(testpath.P(`C:\a`)); (msg != "") != wantRefused {
			t.Errorf("picking %q: %q", picked, msg)
		}
	}
	if got := h.store.Get().Exclude; len(got) != 1 || got[0] != testpath.P(`C:\a\node_modules`) {
		t.Fatalf("exclude = %v", got)
	}
	if msg := h.svc.StopLeavingOut(testpath.P(`C:\a\node_modules`)); msg != "" || len(h.store.Get().Exclude) != 0 {
		t.Fatalf("StopLeavingOut: %q, exclude %v", msg, h.store.Get().Exclude)
	}
}
