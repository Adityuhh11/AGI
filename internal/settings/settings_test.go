package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agi/internal/config"
)

func open(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "AGI", "config.toml")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func TestUpdateWritesTheFileAndChangesWhatGetReturns(t *testing.T) {
	s, path := open(t)
	if err := s.Update(func(c *config.Config) { c.Hotkey = "Ctrl+Alt+K"; c.Roots = []string{`C:\Docs`} }); err != nil {
		t.Fatal(err)
	}
	if got := s.Get(); got.Hotkey != "Ctrl+Alt+K" || len(got.Roots) != 1 {
		t.Fatalf("Get() = %+v", got)
	}
	onDisk, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.Hotkey != "Ctrl+Alt+K" || len(onDisk.Roots) != 1 {
		t.Fatalf("the file holds %+v", onDisk)
	}
}

func TestAnInvalidChangeLeavesTheFileAndTheMemoryAlone(t *testing.T) {
	s, path := open(t)
	before, _ := os.ReadFile(path)
	err := s.Update(func(c *config.Config) { c.Hotkey = ""; c.Roots = []string{"relative"} })
	if err == nil || !strings.Contains(err.Error(), "hotkey") {
		t.Fatalf("Update error = %v, want one about the hotkey", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) || s.Get().Hotkey != config.DefaultHotkey {
		t.Fatalf("a rejected change leaked: file changed %v, hotkey %q", string(before) != string(after), s.Get().Hotkey)
	}
}

func TestGetGivesACopyNobodyCanUseToChangeTheStore(t *testing.T) {
	s, _ := open(t)
	if err := s.Update(func(c *config.Config) { c.Roots = []string{`C:\Docs`} }); err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	got.Roots[0] = `C:\Elsewhere`
	if s.Get().Roots[0] != `C:\Docs` {
		t.Fatal("changing the copy changed the store")
	}
}

func TestSubscribersSeeEachChangeOnceInOrderWithBeforeAndAfter(t *testing.T) {
	s, _ := open(t)
	var seen []string
	s.Subscribe(func(old, updated config.Config) { seen = append(seen, old.Hotkey+">"+updated.Hotkey) })
	s.Subscribe(func(_, updated config.Config) { seen = append(seen, "second:"+updated.Hotkey) })
	for _, key := range []string{"Ctrl+Alt+A", "Ctrl+Alt+B"} {
		if err := s.Update(func(c *config.Config) { c.Hotkey = key }); err != nil {
			t.Fatal(err)
		}
	}
	want := "Alt+Shift+Space>Ctrl+Alt+A|second:Ctrl+Alt+A|Ctrl+Alt+A>Ctrl+Alt+B|second:Ctrl+Alt+B"
	if got := strings.Join(seen, "|"); got != want {
		t.Fatalf("subscribers saw %s, want %s", got, want)
	}
}

func TestAChangeThatChangesNothingAnnouncesNothing(t *testing.T) {
	s, _ := open(t)
	calls := 0
	s.Subscribe(func(_, _ config.Config) { calls++ })
	if err := s.Update(func(c *config.Config) { c.Hotkey = config.DefaultHotkey }); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("subscribers were called %d times for a no-op", calls)
	}
}

func TestAStaleTempFileFromACrashDoesNotBlockTheNextSave(t *testing.T) {
	s, path := open(t)
	if err := os.WriteFile(path+".tmp", []byte("half a fi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(path); err != nil {
		t.Fatalf("the real file must be untouched by a crashed save: %v", err)
	}
	if err := s.Update(func(c *config.Config) { c.Hotkey = "Ctrl+Alt+K" }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("the temp file was left behind: %v", err)
	}
}

func TestAnUnreadableFileIsKeptAsideAndTheDefaultsAreUsed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("hotkey = 'Ctrl+Alt+K'\nfrom_the_future = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if store.Get().Hotkey != config.DefaultHotkey {
		t.Errorf("hotkey = %q, want the default", store.Get().Hotkey)
	}
	kept, err := os.ReadFile(filepath.Join(filepath.Dir(path), "config.unreadable.toml"))
	if err != nil || !strings.Contains(string(kept), "from_the_future") {
		t.Errorf("the old file was not kept: %q, %v", kept, err)
	}
}
