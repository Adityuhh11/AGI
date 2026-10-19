package enginesetup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Downloader fetches pinned artifacts. A download goes to <dest>.part and is renamed only once its size
// and SHA-256 match, so a file under its real name is always a whole, checked one; a part that is left
// behind (cancelled, offline) is continued with a Range request.
type Downloader struct {
	Client *http.Client
}

// Download makes dest hold the artifact. progress (may be nil) is called with the bytes done and the total.
func (d Downloader) Download(ctx context.Context, a Artifact, dest string, progress func(done, total int64)) error {
	if progress == nil {
		progress = func(int64, int64) {}
	}
	if matches(dest, a) {
		progress(a.Size, a.Size)
		return nil
	}
	_ = os.Remove(dest) // present but wrong: start again
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(dest), err)
	}
	part := dest + ".part"
	have := fileSize(part)
	if have > a.Size {
		_ = os.Remove(part)
		have = 0
	}
	if have < a.Size {
		var err error
		if have, err = d.fetch(ctx, a, part, have, progress); err != nil {
			return err
		}
	}
	if have != a.Size || !matches(part, a) {
		_ = os.Remove(part)
		return fmt.Errorf("%s: the downloaded file is not the one that was expected (size or checksum differ); it was deleted", a.Name)
	}
	if err := os.Rename(part, dest); err != nil {
		return fmt.Errorf("finish %s: %w", a.Name, err)
	}
	return nil
}

// fetch appends to part from offset have and returns the size part has afterwards.
func (d Downloader) fetch(ctx context.Context, a Artifact, part string, have int64, progress func(done, total int64)) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return have, err
	}
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
	}
	client := d.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return have, fmt.Errorf("download %s: %w", a.Name, err)
	}
	defer resp.Body.Close()
	flags := os.O_WRONLY | os.O_CREATE
	switch resp.StatusCode {
	case http.StatusPartialContent:
		flags |= os.O_APPEND
	case http.StatusOK: // the server ignored the range (or there was nothing to resume): the body is the whole file
		flags |= os.O_TRUNC
		have = 0
	default:
		return have, fmt.Errorf("download %s: the server answered %s", a.Name, resp.Status)
	}
	file, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return have, fmt.Errorf("write %s: %w", part, err)
	}
	defer file.Close()
	progress(have, a.Size)
	buf := make([]byte, 256<<10)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := file.Write(buf[:n]); err != nil {
				return have, fmt.Errorf("write %s: %w", part, err)
			}
			have += int64(n)
			progress(have, a.Size)
		}
		if readErr == io.EOF {
			return have, nil
		}
		if readErr != nil {
			return have, fmt.Errorf("download %s: %w", a.Name, readErr) // the part stays, to be continued
		}
	}
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// matches reports whether the file at path has the artifact's size and SHA-256.
func matches(path string, a Artifact) bool {
	if fileSize(path) != a.Size {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return false
	}
	return strings.EqualFold(hex.EncodeToString(sum.Sum(nil)), a.SHA256)
}

// Unzip extracts the archive into dir. An entry that would land outside dir is refused.
func Unzip(archive, dir string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(archive), err)
	}
	defer reader.Close()
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for _, entry := range reader.File {
		target := filepath.Join(root, filepath.FromSlash(entry.Name))
		if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
			return fmt.Errorf("%s holds a path outside the folder it is extracted to: %q", filepath.Base(archive), entry.Name)
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := extractFile(entry, target); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(entry *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := entry.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("extract %s: %w", entry.Name, err)
	}
	return out.Close()
}
