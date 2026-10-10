package store

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// BinDir is where every Facile tool is installed. One directory for the whole
// suite, never sudo, never outside $HOME by default.
func BinDir() string {
	if dir := os.Getenv("FACILE_BIN_DIR"); dir != "" {
		return strings.TrimRight(dir, "/")
	}
	return filepath.Join(userHome(), ".local", "bin")
}

// ConfigDir holds facile's own state, not the tools' configs.
func ConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "facile")
	}
	return filepath.Join(userHome(), ".config", "facile")
}

// CacheDir holds the refreshed tool catalog.
func CacheDir() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "facile")
	}
	return filepath.Join(userHome(), ".cache", "facile")
}

// CatalogPath is the on-disk cache of the remote tool catalog.
func CatalogPath() string { return filepath.Join(CacheDir(), "tools.yml") }

// SourcesPaths returns the ordered list of sources file paths in priority
// order (first wins): ~/.facile.yml, ~/.config/facile/sources.yml.
func SourcesPaths() []string {
	home := userHome()
	return []string{
		filepath.Join(home, ".facile.yml"),
		filepath.Join(home, ".config", "facile", "sources.yml"),
	}
}

// SourcesPath returns the path of the first sources file that exists, in
// priority order. Returns empty string if neither exists.
func SourcesPath() string {
	for _, p := range SourcesPaths() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Tilde shortens a path under $HOME for display.
func Tilde(path string) string {
	h := userHome()
	if h != "" && strings.HasPrefix(path, h) {
		return "~" + strings.TrimPrefix(path, h)
	}
	return path
}

// OnPath reports whether dir is listed in $PATH.
func OnPath(dir string) bool {
	return slices.Contains(filepath.SplitList(os.Getenv("PATH")), dir)
}

func userHome() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}
