package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPathIsUnderTheUsersConfigFolder(t *testing.T) {
	t.Setenv("APPDATA", `C:\Users\someone\AppData\Roaming`)
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := `C:\Users\someone\AppData\Roaming\AGI\config.toml`; got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestLoadCreatesTheFileWithDefaultsWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGI", "config.toml")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Hotkey != DefaultHotkey {
		t.Fatalf("hotkey = %q, want the default %q", got.Hotkey, DefaultHotkey)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the config file was not created: %v", err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatalf("the created file does not load: %v", err)
	}
	if again.Hotkey != got.Hotkey || strings.Join(again.Roots, "|") != strings.Join(got.Roots, "|") ||
		again.Sidecar != got.Sidecar {
		t.Fatalf("round trip changed the config: %+v became %+v", got, again)
	}
}

func TestLoadKeepsDefaultsForKeysThatAreNotSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "hotkey = \"Ctrl+Alt+K\"\n")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Hotkey != "Ctrl+Alt+K" {
		t.Fatalf("hotkey = %q, want Ctrl+Alt+K", got.Hotkey)
	}
	if want := Default(); got.Sidecar != want.Sidecar {
		t.Fatalf("sidecar = %+v, want the default %+v", got.Sidecar, want.Sidecar)
	}
}

func TestLoadReplacesTheRootListWhenItIsSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "roots = ['C:\\Docs', 'D:\\Photos']\n")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Roots) != 2 || got.Roots[0] != `C:\Docs` || got.Roots[1] != `D:\Photos` {
		t.Fatalf("roots = %v", got.Roots)
	}
}

func TestLoadRejectsBadFiles(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{"not toml", "hotkey = = =", "parse"},
		{"unknown key", "hotkeys = 'x'\n", "parse"},
		{"unknown table key", "[sidecar]\nfolder = 'x'\n", "parse"},
		{"wrong type", "roots = 'C:\\Docs'\n", "parse"},
		{"empty hotkey", "hotkey = ''\n", "hotkey must not be empty"},
		{"relative root", "roots = ['docs']\n", "must be an absolute path"},
		{"duplicate root", "roots = ['C:\\Docs', 'c:\\docs\\']\n", "repeats"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			writeFile(t, path, tt.content)
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load error = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestSaveCreatesMissingFolders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "config.toml")
	if err := (Config{Hotkey: "Alt+Shift+Space"}).Save(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestSourceIDsAreStableCaseInsensitiveAndAcceptedByTheEngine(t *testing.T) {
	enginePattern := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	tests := []struct {
		a, b string
		same bool
	}{
		{`C:\Users\me\Docs`, `C:\Users\me\Docs`, true},
		{`C:\Users\me\Docs`, `c:\users\ME\docs`, true},
		{`C:\Users\me\Docs`, `C:\Users\me\Docs\`, true},
		{`C:\Users\me\Docs`, `C:\Users\me\Docs\.`, true},
		{`C:\Users\me\Docs`, `C:\Users\me\Photos`, false},
		{`C:\Users\me\Docs`, `D:\Users\me\Docs`, false},
	}
	for _, tt := range tests {
		a, b := SourceID(tt.a), SourceID(tt.b)
		if (a == b) != tt.same {
			t.Errorf("SourceID(%q) = %s, SourceID(%q) = %s, same = %v, want %v", tt.a, a, tt.b, b, a == b, tt.same)
		}
		if !enginePattern.MatchString(a) {
			t.Errorf("SourceID(%q) = %q does not fit the engine's source id rule", tt.a, a)
		}
	}
}

func TestDefaultsPointAtTheRepositoryWhenItIsFound(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "sidecar", "pyproject.toml"), "[project]\n")
	deep := filepath.Join(repo, "bin", "nested")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findRepoRoot(t.TempDir(), deep); got != repo {
		t.Fatalf("findRepoRoot = %q, want %q (the second start folder is inside the repository)", got, repo)
	}
	cfg := defaultsFor(repo)
	if want := filepath.Join(repo, "Sample Data"); len(cfg.Roots) != 1 || cfg.Roots[0] != want {
		t.Fatalf("roots = %v, want [%s]", cfg.Roots, want)
	}
	if want := filepath.Join(repo, "sidecar"); cfg.Sidecar.Dir != want {
		t.Fatalf("sidecar dir = %q, want %q", cfg.Sidecar.Dir, want)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("the defaults are invalid: %v", err)
	}
}

func TestDefaultsWithoutARepositoryHaveNoRootsAndStillValidate(t *testing.T) {
	if got := findRepoRoot(t.TempDir()); got != "" {
		t.Fatalf("findRepoRoot found %q in an empty folder", got)
	}
	cfg := defaultsFor("")
	if len(cfg.Roots) != 0 || cfg.Sidecar.Dir != "" || cfg.Hotkey != DefaultHotkey {
		t.Fatalf("unexpected defaults without a repository: %+v", cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}
