package enginesetup

import (
	"os"
	"path/filepath"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLocateUsesTheCacheFolderUnlessTheEnvironmentSaysOtherwise(t *testing.T) {
	t.Setenv("AGI_ENGINE_DIR", "")
	t.Setenv("LOCALAPPDATA", `C:\Users\someone\AppData\Local`)
	got, err := Locate()
	if err != nil {
		t.Fatal(err)
	}
	if want := `C:\Users\someone\AppData\Local\AGI\engine`; got.Root != want {
		t.Fatalf("Root = %q, want %q", got.Root, want)
	}
	t.Setenv("AGI_ENGINE_DIR", `D:\dev\engine`)
	if got, _ = Locate(); got.Root != `D:\dev\engine` {
		t.Fatalf("Root = %q, want the override", got.Root)
	}
}

func TestAnEngineIsInstalledOnlyWhenTheManifestAndThePythonAreBothThere(t *testing.T) {
	l := Layout{Root: t.TempDir()}
	if l.Installed() {
		t.Fatal("an empty folder counts as installed")
	}
	touch(t, l.Python())
	if l.Installed() {
		t.Fatal("an environment without a finished install counts as installed")
	}
	touch(t, l.Manifest())
	if !l.Installed() {
		t.Fatal("a finished install is not recognised")
	}
	if err := os.Remove(l.Python()); err != nil {
		t.Fatal(err)
	}
	if l.Installed() {
		t.Fatal("a manifest alone counts as installed")
	}
}
