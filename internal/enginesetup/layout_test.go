package enginesetup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"omni/internal/config"
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
	t.Setenv("OMNI_ENGINE_DIR", "")
	data, err := config.DataDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Locate("")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(data, "engine"); got.Root != want {
		t.Fatalf("Root = %q, want %q", got.Root, want)
	}
	t.Setenv("OMNI_ENGINE_DIR", `D:\dev\engine`)
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
	t.Setenv("OMNI_ENGINE_DIR", "")
	got, err := Locate(`D:\Files\Omni`)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != `D:\Files\Omni\engine` || got.Index() != `D:\Files\Omni\memory` {
		t.Fatalf("layout = %+v", got)
	}
	env := strings.Join(got.Env(), "|")
	if !strings.Contains(env, `OMNI_MEMORY_DIR=D:\Files\Omni\memory`) || !strings.Contains(env, `OMNI_MODEL_DIR=D:\Files\Omni\engine\models`) {
		t.Errorf("env = %s", env)
	}
	t.Setenv("OMNI_ENGINE_DIR", "")
	data, _ := config.DataDir()
	def, _ := Locate("")
	if def.Memory != "" {
		t.Errorf("the default keeps no folder of its own for the index: %+v", def)
	}
	if def.Index() != filepath.Join(data, "memory") || !strings.Contains(strings.Join(def.Env(), "|"), "OMNI_MEMORY_DIR="+def.Index()) {
		t.Errorf("default index = %q, env %v: the engine must be told where it is", def.Index(), def.Env())
	}
	t.Setenv("OMNI_ENGINE_DIR", `E:\dev\engine`)
	if over, _ := Locate(`D:\Files\Omni`); over.Root != `E:\dev\engine` {
		t.Errorf("OMNI_ENGINE_DIR must win: %+v", over)
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
