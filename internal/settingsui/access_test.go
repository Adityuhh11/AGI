package settingsui

import (
	"sync"
	"testing"
	"time"

	"omni/internal/indexing"
	"omni/internal/sidecar"
	"omni/internal/testpath"
)

// asked records the folders the service read to find out whether the system lets it, in the order it did.
type asked struct {
	mu      sync.Mutex
	folders []string
}

func (a *asked) blocked(denied ...string) func(string) bool {
	return func(root string) bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.folders = append(a.folders, root)
		for _, d := range denied {
			if root == d {
				return true
			}
		}
		return false
	}
}

func (a *asked) wait(t *testing.T, n int) []string {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		a.mu.Lock()
		if len(a.folders) >= n {
			got := append([]string(nil), a.folders...)
			a.mu.Unlock()
			return got
		}
		a.mu.Unlock()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	t.Fatalf("only %v were read, want %d", a.folders, n)
	return nil
}

func TestAFolderTheSystemWillNotLetOmniReadSaysSoAndOthersDoNot(t *testing.T) {
	h := newHarness(t)
	denied, fine := testpath.P(`C:\Users\me\Documents`), testpath.P(`C:\Users\me\Pictures`)
	h.index.folders = []indexing.FolderStatus{{Root: denied, State: indexing.UpToDate}, {Root: fine, State: indexing.UpToDate, Files: 3}}
	h.svc.o.Engine = func() sidecar.Status { return sidecar.Status{State: sidecar.StateReady} }
	var a asked
	h.svc.blocked = a.blocked(denied)
	got := h.svc.Search().Folders
	if !got[0].Blocked || got[0].Text != blockedText {
		t.Errorf("the refused folder says %+v", got[0])
	}
	if got[1].Blocked || got[1].Text != "3 files searchable" {
		t.Errorf("the other folder says %+v", got[1])
	}
}

func TestOnlyAFolderThatIsNewlyChosenIsReadAndReadOnceEach(t *testing.T) {
	h := newHarness(t)
	old, fresh := testpath.P(`C:\Old`), testpath.P(`C:\Fresh`)
	if msg := h.svc.SetRoots([]string{old}); msg != "" {
		t.Fatal(msg)
	}
	var a asked
	h.svc.blocked = a.blocked()
	if msg := h.svc.SetRoots([]string{old, fresh}); msg != "" {
		t.Fatal(msg)
	}
	if got := a.wait(t, 1); len(got) != 1 || got[0] != fresh {
		t.Fatalf("read %v, want only %s: the folder that was already chosen has been asked about", got, fresh)
	}
}
