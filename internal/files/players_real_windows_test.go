package files

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakePlayer writes the arguments it is started with, one a line, to the file named by FAKEPLAYER_OUT.
const fakePlayer = `package main

import (
	"os"
	"strings"
)

func main() {
	_ = os.WriteFile(os.Getenv("FAKEPLAYER_OUT"), []byte(strings.Join(os.Args[1:], "\n")), 0o644)
}
`

// The real Windows path: a program called vlc.exe on PATH is found, and started with the arguments VLC documents for a moment,
// for a file whose name has spaces in it, without the launcher waiting for it.
func TestARealProgramOnPathIsFoundAndStartedAtAMoment(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the Go tool is needed to build a stand-in player")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	if err := os.WriteFile(source, []byte(fakePlayer), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	build := exec.Command(goTool, "build", "-o", filepath.Join(bin, "vlc.exe"), source)
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the stand-in player: %v\n%s", err, out)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	result := filepath.Join(dir, "args.txt")
	t.Setenv("FAKEPLAYER_OUT", result)

	shell := windowsShell{}
	player, ok := shell.SeekPlayer()
	if !ok || player.Name != "VLC" || !strings.EqualFold(player.Exe, filepath.Join(bin, "vlc.exe")) {
		t.Fatalf("SeekPlayer = %+v, %v", player, ok)
	}
	talk := filepath.Join(dir, "My Talks", "sleep 1.mp3")
	if err := shell.Player(player, talk, 75); err != nil {
		t.Fatal(err)
	}
	var got []byte
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if got, err = os.ReadFile(result); err == nil && len(got) > 0 {
			break
		}
	}
	if want := "--start-time=75.0\n" + talk; string(got) != want {
		t.Fatalf("the player was started with %q, want %q", got, want)
	}
}
