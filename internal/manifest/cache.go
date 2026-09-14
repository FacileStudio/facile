package manifest

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	remoteURL   = "https://raw.githubusercontent.com/FacileStudio/facile/main/internal/manifest/tools.yml"
	cacheMaxAge = 24 * time.Hour
	fetchLimit  = 1 << 20
)

// fetch pulls the remote catalog. A slow or broken network surfaces here and
// the loader falls back to the embedded copy.
func fetch() ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(remoteURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog returned %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, fetchLimit))
}

// fresh keeps a cached catalog for a day, but never past an upgrade of facile
// itself. A new binary carries a new embedded catalog, and serving a cache
// written by the old one would hide exactly the change the user just installed
// — a tool that gained a login flow would keep asking for a pasted token.
func fresh(path string) bool {
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) >= cacheMaxAge {
		return false
	}
	self, err := os.Executable()
	if err != nil {
		return true
	}
	binary, err := os.Stat(self)
	if err != nil {
		return true
	}
	return !binary.ModTime().After(info.ModTime())
}

func writeCache(path string, raw []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	os.WriteFile(path, raw, 0o644)
}
