// Package atomicfile replaces a file so that a crash or a power cut leaves either the old content or the new, never a
// half-written or empty file.
package atomicfile

import (
	"os"
)

// Write puts data in the file at path, readable by the user only. It is written to path+".tmp" and flushed to the disk
// before the rename, because a rename that reaches the disk ahead of the data leaves an empty file after a power cut. A
// ".tmp" left by a crash is overwritten by the next write. The folder must exist, and two writes to one path must not overlap.
func Write(path string, data []byte) error {
	temp := path + ".tmp"
	file, err := os.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temp, path)
	}
	if err != nil {
		_ = os.Remove(temp)
	}
	return err
}
