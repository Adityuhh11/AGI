package enginesetup

import (
	"os"
	"path/filepath"
	"strings"
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
	got, err := Locate("")
	if err != nil {
		t.Fatal(err)
	}
	if want := `C:\Users\someone\AppData\Local\AGI\engine`; got.Root != want {
		t.Fatalf("Root = %q, want %q", got.Root, want)
	}
	t.Setenv("AGI_ENGINE_DIR", `D:\dev\engine`)
	if got, _ = Locate(""); got.Root != `D:\dev\engine` {
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

func TestAChosenFolderHoldsTheEngineAndTheIndexAndTheEnvironmentStillWins(t *testing.T) {
	t.Setenv("AGI_ENGINE_DIR", "")
	got, err := Locate(`D:\Files\AGI`)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != `D:\Files\AGI\engine` || got.Index() != `D:\Files\AGI\memory` {
		t.Fatalf("layout = %+v", got)
	}
	env := strings.Join(got.Env(), "|")
	if !strings.Contains(env, `AGI_MEMORY_DIR=D:\Files\AGI\memory`) || !strings.Contains(env, `AGI_MODEL_DIR=D:\Files\AGI\engine\models`) {
		t.Errorf("env = %s", env)
	}
	t.Setenv("LOCALAPPDATA", `C:\Users\someone\AppData\Local`)
	t.Setenv("AGI_ENGINE_DIR", "")
	def, _ := Locate("")
	if def.Memory != "" || strings.Contains(strings.Join(def.Env(), "|"), "AGI_MEMORY_DIR") {
		t.Errorf("the default must leave the engine's own index folder alone: %+v", def)
	}
	if def.Index() != `C:\Users\someone\AppData\Local\AGI\memory` {
		t.Errorf("default index = %q", def.Index())
	}
	t.Setenv("AGI_ENGINE_DIR", `E:\dev\engine`)
	if over, _ := Locate(`D:\Files\AGI`); over.Root != `E:\dev\engine` {
		t.Errorf("AGI_ENGINE_DIR must win: %+v", over)
	}
}

func TestAnIndexIsThereOnceItsDatabaseIs(t *testing.T) {
	l := Layout{Root: t.TempDir(), Memory: t.TempDir()}
	if l.HasIndex() {
		t.Fatal("an empty folder holds an index")
	}
	touch(t, filepath.Join(l.Memory, "meta.db"))
	if !l.HasIndex() {
		t.Fatal("the index is not recognised")
	}
}
