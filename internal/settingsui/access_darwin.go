package settingsui

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
)

// blockedText is what a folder says when macOS has not let Omni read it.
const blockedText = "macOS has not let Omni read this folder. Allow Omni in System Settings > Privacy & Security > Files and Folders (or give it Full Disk Access), then press Rescan now in the menu bar."

// blocked says whether macOS refuses this program a folder. Documents, Desktop, Downloads, the media folders, iCloud Drive and removable volumes are protected: the first
// time a program lists one the system asks the person, and until they answer (and if they refuse) the read fails as a permission error. Listing one entry is enough
// to be asked, and is cheap enough to do again each time the page asks how the folders are.
func blocked(root string) bool {
	dir, err := os.Open(root)
	if err == nil {
		defer dir.Close()
		_, err = dir.Readdirnames(1)
	}
	return errors.Is(err, fs.ErrPermission)
}

// openAccessSettings opens System Settings on the page that lists the folders programs may read.
func openAccessSettings() error {
	cmd := exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_FilesAndFolders")
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
