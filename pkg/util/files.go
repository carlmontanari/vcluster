package util

import (
	"os"
	"path/filepath"
	"strings"
)

// ExpandPath expands user home path in provided path.
func ExpandPath(s string) string {
	userPath, _ := os.UserHomeDir()

	return strings.Replace(s, "~", userPath, 1)
}

// FileExists checks if a given file exists (and is not a directory).
func FileExists(s string) bool {
	info, err := os.Stat(s)
	if os.IsNotExist(err) {
		return false
	}

	return !info.IsDir()
}

// ResolveFile resolves provided files absolute path. Failure to resolve the file will return an
// empty string.
func ResolveFile(s string) string {
	expanded := ExpandPath(s)

	if FileExists(expanded) {
		resolved, err := filepath.Abs(expanded)
		if err == nil {
			return resolved
		}
	}

	return ""
}
