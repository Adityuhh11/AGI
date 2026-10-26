package settingsui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agi/internal/config"
	"agi/internal/enginesetup"
)

// Where file search keeps its files. The engine and its models (about 1.1 GB) and the index can be put on another
// drive, but only before the first download: once there is something in a folder, moving it is not done here.

// Place is the install manager as the folder choice uses it; *enginesetup.Manager satisfies it.
type Place interface {
	Layout() enginesetup.Layout
	Move(to enginesetup.Layout) error
}

// DataFolder says where file search keeps its files and whether that can still be changed.
type DataFolder struct {
	// Engine and Index are the folders in use (or, before the download, the ones it will use).
	Engine, Index string
	// Chosen says the person picked a folder; false means the default under %LOCALAPPDATA%.
	Chosen bool
	// CanChange is false once the engine is installed, being installed, or an index is built.
	CanChange bool
}

// dataFolder takes the install state its caller already has: asking again re-reads the whole engine source to see whether it is
// out of date, and the page asks for all this every second.
func (s *Service) dataFolder(cfg config.Config, installed enginesetup.State) DataFolder {
	l := s.o.Place.Layout()
	return DataFolder{
		Engine: l.Root, Index: l.Index(), Chosen: cfg.Search.DataDir != "",
		CanChange: !installed.Installed && !installed.Installing && !l.HasIndex(),
	}
}

// ChooseDataFolder asks for a folder and keeps file search's files in an AGI folder inside it. Cancelling the picker
// changes nothing.
func (s *Service) ChooseDataFolder() string {
	picked, err := s.o.PickFolder()
	if err != nil {
		return fmt.Sprintf("the folder picker failed: %v", err)
	}
	if picked == "" {
		return ""
	}
	return s.setDataDir(dataDirIn(filepath.Clean(picked)))
}

// UseDefaultDataFolder goes back to %LOCALAPPDATA%\AGI.
func (s *Service) UseDefaultDataFolder() string { return s.setDataDir("") }

// dataDirIn is the folder AGI keeps its files in inside a folder the person picked: an AGI folder, unless they picked
// one already called that.
func dataDirIn(picked string) string {
	if strings.EqualFold(filepath.Base(picked), "AGI") {
		return picked
	}
	return filepath.Join(picked, "AGI")
}

func (s *Service) setDataDir(dir string) string {
	layout, err := enginesetup.Locate(dir)
	if err != nil {
		return reason(err)
	}
	if dir != "" {
		if err := writable(dir); err != nil {
			return fmt.Sprintf("AGI cannot keep its files in %s: %v", dir, err)
		}
	}
	if err := s.o.Place.Move(layout); err != nil {
		return reason(err)
	}
	if msg := reason(s.o.Store.Update(func(c *config.Config) { c.Search.DataDir = dir })); msg != "" {
		return msg
	}
	return ""
}

// writable makes the folder and writes a file in it and removes it, so a folder that cannot be used is refused here
// and not in the middle of a 1 GB download.
func writable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	probe, err := os.CreateTemp(dir, ".agi-write-test-*")
	if err != nil {
		return err
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		return errors.Join(err, os.Remove(name))
	}
	return os.Remove(name)
}
