package files

import (
	"errors"
	"reflect"
	"testing"
)

func nowhere(string) (string, error) { return "", errors.New("not on PATH") }

func TestMacPlayersAreFoundInTheOrderVLCIinaThenMpv(t *testing.T) {
	onPath := func(exe string) (string, error) {
		if exe == "mpv" {
			return "/opt/homebrew/bin/mpv", nil
		}
		return "", errors.New("no")
	}
	p, ok := findPlayer(onPath, func(path string) bool { return path == "/Applications/IINA.app/Contents/MacOS/iina-cli" }, []string{"/Applications"})
	if !ok || p.Name != "IINA" {
		t.Fatalf("found %+v, %v: IINA is installed and comes before mpv", p, ok)
	}
	p, ok = findPlayer(onPath, func(string) bool { return false }, []string{"/Applications"})
	if !ok || p.Name != "mpv" || p.Exe != "/opt/homebrew/bin/mpv" {
		t.Fatalf("found %+v, %v", p, ok)
	}
	if _, ok := findPlayer(nowhere, func(string) bool { return false }, []string{"/Applications"}); ok {
		t.Fatal("no player installed must find none")
	}
}

func TestIinaIsAskedForAMomentThroughMpvsOption(t *testing.T) {
	got := Player{Name: "IINA", kind: iina}.args("/Users/me/clip.mp4", 90.5)
	if want := []string{"--mpv-start=90.5", "/Users/me/clip.mp4"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %q, want %q", got, want)
	}
}
