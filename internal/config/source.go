package config

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

// SourceID names a root in the memory engine. It is derived from the path (case-insensitive,
// as Windows paths are) so the same folder always maps to the same source, and it fits the
// engine's source id rule (letters, digits, dot, dash, underscore; at most 64 characters).
func SourceID(root string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(root))))
	return "root-" + hex.EncodeToString(sum[:])[:10]
}
