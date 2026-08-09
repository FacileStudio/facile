package store

import (
	"os"
	"path/filepath"
	"strings"
)

// BinDir is where every Facile tool is installed. One directory for the whole
// suite, never sudo, never outside $HOME by default.
func BinDir() string {
	if dir := os.Getenv("FACILE_BIN_DIR"); dir != "" {
		return strings.TrimRight(dir, "/")
	}
	return filepath.Join(home(), ".local", "bin")
}

// ConfigDir holds facile's own state, not the tools' configs.
func ConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "facile")
	}
	return filepath.Join(home(), ".config", "facile")
}

// CacheDir holds the refreshed tool catalog.
func CacheDir() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "facile")
	}
	return filepath.Join(home(), ".cache", "facile")
}

// CatalogPath is the on-disk cache of the remote tool catalog.
func CatalogPath() string { return filepath.Join(CacheDir(), "tools.yml") }

// Tilde shortens a path under $HOME for display.
func Tilde(path string) string {
	h := home()
	if h != "" && strings.HasPrefix(path, h) {
		return "~" + strings.TrimPrefix(path, h)
	}
	return path
}

// OnPath reports whether dir is listed in $PATH.
func OnPath(dir string) bool {
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if entry == dir {
			return true
		}
	}
	return false
}

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}
