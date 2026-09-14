package credstore

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
)

const (
	defaultMode    fs.FileMode = 0o600
	defaultDirMode fs.FileMode = 0o700
)

// writeFile applies sets and deletes to the tool's config file and returns the
// path it wrote. When the store is marked preserve, the existing file is read
// first: nuage keeps sync_dir and ignore_patterns in the same file as its
// token, and a wholesale overwrite silently resets a user's sync directory.
func writeFile(s *manifest.Store, sets []field, deletes []string) (string, error) {
	path, err := Expand(s.Path)
	if err != nil {
		return "", err
	}

	mode := pickMode(s.Mode, defaultMode)
	if err := os.MkdirAll(filepath.Dir(path), pickMode(s.DirMode, defaultDirMode)); err != nil {
		return "", fmt.Errorf("cannot create %s — check the directory's permissions", store.Tilde(filepath.Dir(path)))
	}

	var existing []byte
	if s.Preserve {
		if raw, err := os.ReadFile(path); err == nil {
			existing = raw
		}
	}

	body, err := apply(s.Format, existing, sets, deletes)
	if err != nil {
		return "", fmt.Errorf("cannot update %s — %s", store.Tilde(path), err)
	}
	if err := createAt(path, body, mode); err != nil {
		return "", fmt.Errorf("cannot write %s — check the file's permissions", store.Tilde(path))
	}
	return path, nil
}

// createAt opens the file at its final mode rather than writing it and
// chmodding afterwards, which would leave a window where a credential sits at
// 0644. The explicit Chmod only matters for a file that already existed, whose
// mode the open flags do not touch.
func createAt(path string, body []byte, mode fs.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := file.Chmod(mode); err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		return err
	}
	return file.Sync()
}

func pickMode(raw uint32, fallback fs.FileMode) fs.FileMode {
	if raw == 0 {
		return fallback
	}
	return fs.FileMode(raw)
}

// apply dispatches to the format's rewrite. The formats are siblings so none of
// them grows the read/write paths beyond the line editor their format needs.
func apply(format string, src []byte, sets []field, deletes []string) ([]byte, error) {
	switch format {
	case "yaml", "yml":
		return applyYAML(src, sets, deletes)
	case "json":
		return applyJSON(src, sets, deletes)
	case "toml":
		return applyTOML(src, sets, deletes)
	default:
		return nil, fmt.Errorf("unknown config format %q", format)
	}
}

// read returns the value for key from src, or "" when the key is absent.
func read(format string, src []byte, key string) (string, error) {
	switch format {
	case "yaml", "yml":
		return readYAML(src, key)
	case "json":
		return readJSON(src, key)
	case "toml":
		return readTOML(src, key)
	}
	return "", nil
}
