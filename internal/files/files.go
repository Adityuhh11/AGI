// Package files opens search results: it hands a file to the right program or shows it in
// Explorer, and refuses anything that is not inside a folder the launcher indexes.
//
// The rule that matters: a file is opened with its default program only if that is known to be
// harmless (documents and pictures). Windows' default action for a script (.js, .bat, .ps1,
// .hta, ...) is to RUN it, and a code search result must never do that, so everything else
// opens in a text editor.
package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// opener launches programs; the Windows implementation is in shell_windows.go.
type opener interface {
	// Default opens path with the program Windows has chosen for its type.
	Default(path string) error
	// Editor opens path in a text editor, at line if the editor supports it (line > 0).
	Editor(path string, line int) error
	// Reveal shows path selected in an Explorer window.
	Reveal(path string) error
}

// Service is bound to the frontend; its exported methods become TypeScript functions.
type Service struct {
	roots   func() []string
	dismiss func()
	shell   opener
}

// New returns a service that only touches files under the folders roots returns (asked on every
// call, so folders added in the settings count at once). dismiss is called after a file has been
// handed over, to close the search bar.
func New(roots func() []string, dismiss func()) *Service {
	return newService(roots, dismiss, windowsShell{})
}

func newService(roots func() []string, dismiss func(), shell opener) *Service {
	return &Service{roots: roots, dismiss: dismiss, shell: shell}
}

// Open opens a search result: documents and pictures in their default program, everything else
// in an editor, at line when it is positive.
func (s *Service) Open(path string, line int) error {
	if err := s.check(path); err != nil {
		return err
	}
	var err error
	if opensInDefaultApp(path) {
		err = s.shell.Default(path)
	} else {
		err = s.shell.Editor(path, line)
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	s.dismiss()
	return nil
}

// Reveal shows a search result in Explorer.
func (s *Service) Reveal(path string) error {
	if err := s.check(path); err != nil {
		return err
	}
	if err := s.shell.Reveal(path); err != nil {
		return fmt.Errorf("reveal %s: %w", filepath.Base(path), err)
	}
	s.dismiss()
	return nil
}

// check refuses paths outside the indexed folders and files that are gone.
func (s *Service) check(path string) error {
	if !inside(s.roots(), path) {
		return errors.New("that file is not inside a folder the launcher searches")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("that file is no longer there: %w", err)
	}
	return nil
}

// inside reports whether path is one of roots or lies below one (case-insensitively, as
// Windows paths are), after cleaning away "." and ".." parts.
func inside(roots []string, path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	path = strings.ToLower(filepath.Clean(path))
	for _, root := range roots {
		root = strings.ToLower(filepath.Clean(root))
		rel, err := filepath.Rel(root, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// defaultAppExtensions are the types that are safe to hand to whatever Windows associates with
// them: documents and pictures. Macro-enabled Office types (.docm, .xlsm) and old binary Office
// formats are not here on purpose.
var defaultAppExtensions = map[string]bool{
	".pdf": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true,
	".tif": true, ".tiff": true, ".heic": true, ".avif": true,
	".docx": true, ".xlsx": true, ".pptx": true, ".odt": true,
}

func opensInDefaultApp(path string) bool {
	return defaultAppExtensions[strings.ToLower(filepath.Ext(path))]
}
