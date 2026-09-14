package cmd

import (
	"os"
	"path/filepath"
)

// executable reports the running binary's resolved path, or "" when the
// platform cannot answer.
func executable() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Clean(path)
}

// realPath resolves a path's symlinks, falling back to the path itself when the
// file is gone. Comparing two binaries by real path is what tells an update
// that "another copy on PATH" is actually the same file under a symlink.
func realPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}
