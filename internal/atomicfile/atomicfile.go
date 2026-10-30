// Package atomicfile replaces a file so that a crash or a power cut leaves either the old content or the new, never a
// half-written or empty file.
package atomicfile

import (
	"os"
	"path/filepath"
)

// Write puts data in the file at path, readable by the user only. It is written to a file of its own beside path and flushed
// to the disk before the rename, because a rename that reaches the disk ahead of the data leaves an empty file after a power
// cut. The folder must exist.
func Write(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	_, err = temp.Write(data)
	if err == nil {
		err = temp.Sync()
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		_ = os.Remove(name)
	}
	return err
}
