package enginesetup

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Untar extracts a .tar.gz into dir; macOS releases come that way. strip drops the one folder the archive wraps everything in
// ("llama-b11719/llama-server" lands as dir/llama-server). It keeps the executable bit and the symlinks (a library's version
// links), and refuses an entry, or a link, that would land outside dir.
func Untar(archive, dir string, strip bool) error {
	file, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(archive), err)
	}
	defer file.Close()
	zipped, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(archive), err)
	}
	defer zipped.Close()
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	reader := tar.NewReader(zipped)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", filepath.Base(archive), err)
		}
		name := path.Clean(header.Name)
		if path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") { // checked before the wrapping folder is dropped, which could hide it
			return fmt.Errorf("%s holds a path outside the folder it is extracted to: %q", filepath.Base(archive), header.Name)
		}
		if strip {
			_, rest, found := strings.Cut(name, "/")
			if !found {
				continue // the wrapping folder itself
			}
			name = rest
		}
		target := filepath.Join(root, filepath.FromSlash(name))
		if !within(root, target) {
			return fmt.Errorf("%s holds a path outside the folder it is extracted to: %q", filepath.Base(archive), header.Name)
		}
		// A link earlier in the archive can make a name that looks inside the folder reach outside it, so where the entry
		// really lands is judged after the links it passes through are followed.
		realDir, err := realParent(realRoot, target)
		if err != nil {
			return fmt.Errorf("%s holds a path outside the folder it is extracted to: %q", filepath.Base(archive), header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			err = writeEntry(target, reader, os.FileMode(header.Mode).Perm()|0o600)
		case tar.TypeSymlink:
			if path.IsAbs(header.Linkname) || !within(realRoot, filepath.Join(realDir, filepath.FromSlash(header.Linkname))) {
				return fmt.Errorf("%s holds a link that points outside the folder it is extracted to: %q", filepath.Base(archive), header.Name)
			}
			if err = os.MkdirAll(filepath.Dir(target), 0o755); err == nil {
				_ = os.Remove(target)
				err = os.Symlink(header.Linkname, target)
			}
		case tar.TypeXGlobalHeader:
			// extended attributes of the archive itself, not an entry
		default:
			return fmt.Errorf("%s holds an entry of a kind this does not unpack (%q)", filepath.Base(archive), header.Name)
		}
		if err != nil {
			return fmt.Errorf("extract %s: %w", header.Name, err)
		}
	}
}

// realParent is the folder target sits in once every link on the way is followed, which must be inside realRoot. Folders that
// do not exist yet are not links, so the nearest one that exists decides.
func realParent(realRoot, target string) (string, error) {
	dir := filepath.Dir(target)
	var missing []string
	for {
		real, err := filepath.EvalSymlinks(dir)
		if err == nil {
			real = filepath.Join(append([]string{real}, missing...)...)
			if !within(realRoot, real) {
				return "", fmt.Errorf("%s is outside %s", real, realRoot)
			}
			return real, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", err
		}
		missing = append([]string{filepath.Base(dir)}, missing...)
		dir = parent
	}
}

func within(root, target string) bool {
	return target == root || strings.HasPrefix(target, root+string(filepath.Separator))
}

func writeEntry(target string, in io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	_ = os.Remove(target) // a link of the same name must be replaced, not written through
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
