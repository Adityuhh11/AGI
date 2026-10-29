package apps

import "errors"

// extractIcon is made in milestone M1 (NSWorkspace, cgo). Until then rows have no icon, which the bar already copes with:
// the page shows a letter when /icon answers 404.
func extractIcon(string) ([]byte, error) {
	return nil, errors.New("app icons are not made on macOS yet")
}
