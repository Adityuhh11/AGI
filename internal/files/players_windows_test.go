package files

import (
	"errors"
	"path/filepath"
	"testing"
)

func nowhere(string) (string, error) { return "", errors.New("not on PATH") }
func unregistered(string) string     { return "" }

func TestPlayersAreFoundInTheOrderVLCMpvThenMediaPlayerClassic(t *testing.T) {
	onPath := func(exe string) (string, error) {
		if exe == "mpv.exe" {
			return `C:\tools\mpv.exe`, nil
		}
		return "", errors.New("no")
	}
	// Only mpv is on PATH, but VLC is in its usual folder: VLC is first in the order.
	p, ok := findPlayer(onPath, unregistered, func(string) bool { return true }, []string{`C:\Program Files`})
	if !ok || p.Name != "VLC" {
		t.Fatalf("found %+v, %v", p, ok)
	}
	p, ok = findPlayer(onPath, unregistered, func(string) bool { return false }, []string{`C:\Program Files`})
	if !ok || p.Name != "mpv" || p.Exe != `C:\tools\mpv.exe` || p.kind != mpv {
		t.Fatalf("found %+v, %v", p, ok)
	}
}

func TestAPlayerRegisteredByItsInstallerIsFoundWhereverItIs(t *testing.T) {
	registered := func(exe string) string {
		if exe == "mpc-hc64.exe" {
			return `D:\Video\MPC-HC\mpc-hc64.exe`
		}
		return ""
	}
	exists := func(path string) bool { return path == `D:\Video\MPC-HC\mpc-hc64.exe` }
	p, ok := findPlayer(nowhere, registered, exists, nil)
	if !ok || p.Name != "MPC-HC" || p.kind != mpc {
		t.Fatalf("found %+v, %v", p, ok)
	}
	// a registry entry whose program was deleted is not a player
	if _, ok := findPlayer(nowhere, registered, func(string) bool { return false }, nil); ok {
		t.Fatal("a program that is not there was found")
	}
}

func TestPlayersInTheirUsualFoldersAreFoundAndNoneIsNoneAtAll(t *testing.T) {
	vlcPath := filepath.Join(`C:\Program Files`, `VideoLAN\VLC\vlc.exe`)
	p, ok := findPlayer(nowhere, unregistered, func(path string) bool { return path == vlcPath }, []string{`C:\Program Files`})
	if !ok || p.Name != "VLC" || p.Exe != vlcPath {
		t.Fatalf("found %+v, %v", p, ok)
	}
	if _, ok := findPlayer(nowhere, unregistered, func(string) bool { return false }, []string{`C:\Program Files`}); ok {
		t.Fatal("no player is installed, yet one was found")
	}
}
