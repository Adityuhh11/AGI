package update

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DownloadPrefix is where the zips of this project's releases live. An address from anywhere else is refused. A
// variable only so that a test build can point it at a local server (-ldflags "-X omni/internal/update.DownloadPrefix=...").
var DownloadPrefix = "https://github.com/Adityuhh11/Omni/releases/download/"

// maxZip is the most that is downloaded, and the most that is unpacked from it: the real zip is 5 MB.
const maxZip = 200 << 20

// exeName is the one file taken from a release's zip.
const exeName = "omni.exe"

// countingWriter reports how many bytes went through it.
type countingWriter struct {
	n    int64
	note func(done int64)
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	c.note(c.n)
	return len(p), nil
}

// download fetches the asset into dir and returns the file's path. The file is checked against the asset's SHA-256
// as it arrives; a different one deletes it. progress is told how many bytes of how many have come.
func (i *Installer) download(ctx context.Context, asset Asset, dir string, progress func(done, total int64)) (string, error) {
	if !strings.HasPrefix(asset.URL, i.prefix) {
		return "", fmt.Errorf("the address of the update (%s) is not one of this project's releases, so it is not downloaded", asset.URL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", fmt.Errorf("build the download request: %w", err)
	}
	req.Header.Set("User-Agent", "omni-launcher")
	resp, err := i.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download the update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the download answered %s", resp.Status)
	}
	total := resp.ContentLength
	if total <= 0 {
		total = asset.Size
	}
	if total > i.limit || asset.Size > i.limit {
		return "", fmt.Errorf("the update is %d bytes, more than the %d this program accepts", max(total, asset.Size), i.limit)
	}

	path := filepath.Join(dir, "update.zip")
	out, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create the download file: %w", err)
	}
	sum := sha256.New()
	counter := &countingWriter{note: func(done int64) { progress(done, total) }}
	n, err := io.Copy(io.MultiWriter(out, sum, counter), io.LimitReader(resp.Body, i.limit+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	switch {
	case err != nil:
		return "", fmt.Errorf("download the update: %w", err)
	case n > i.limit:
		return "", fmt.Errorf("the update is bigger than the %d bytes this program accepts", i.limit)
	case asset.Size > 0 && n != asset.Size:
		return "", fmt.Errorf("the download is %d bytes, not the %d of the release: it was cut short", n, asset.Size)
	case hex.EncodeToString(sum.Sum(nil)) != asset.SHA256:
		return "", errors.New("the download does not match the checksum GitHub published for it, so nothing was installed")
	}
	return path, nil
}

// extract writes the one file named omni.exe of the zip to target. Nothing else in the zip is read or written, so a path
// in an entry's name cannot put anything anywhere. The file must look like a Windows program.
func (i *Installer) extract(zipPath, target string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open the downloaded zip: %w", err)
	}
	defer r.Close()
	var entry *zip.File
	for _, f := range r.File {
		if f.Name != exeName {
			continue
		}
		if entry != nil {
			return fmt.Errorf("the zip holds %s twice", exeName)
		}
		entry = f
	}
	switch {
	case entry == nil:
		return fmt.Errorf("the zip has no %s at its top level", exeName)
	case entry.FileInfo().IsDir() || entry.UncompressedSize64 > uint64(i.limit):
		return fmt.Errorf("%s in the zip is not an acceptable file", exeName)
	}
	in, err := entry.Open()
	if err != nil {
		return fmt.Errorf("read %s from the zip: %w", exeName, err)
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(target), err)
	}
	n, err := io.Copy(out, io.LimitReader(in, i.limit+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > i.limit {
		err = errors.New("the program in the zip is bigger than this program accepts")
	}
	if err == nil {
		err = looksLikeAProgram(target)
	}
	if err != nil {
		_ = os.Remove(target)
		return err
	}
	return nil
}

// looksLikeAProgram checks the two bytes every Windows program starts with.
func looksLikeAProgram(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	head := make([]byte, 2)
	if _, err := io.ReadFull(f, head); err != nil || string(head) != "MZ" {
		return fmt.Errorf("%s in the zip is not a Windows program", exeName)
	}
	return nil
}
