package manifest

import (
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

//go:embed tools.yml
var embedded []byte

const (
	remoteURL   = "https://raw.githubusercontent.com/FacileStudio/facile/main/internal/manifest/tools.yml"
	cacheMaxAge = 24 * time.Hour
	fetchLimit  = 1 << 20
)

// Tool is one installable Facile CLI. The field names mirror the config block
// of the per-repo install.sh so a divergence between the two is easy to spot.
type Tool struct {
	Name         string   `yaml:"name"`
	Summary      string   `yaml:"summary"`
	Repo         string   `yaml:"repo"`
	Branch       string   `yaml:"branch"`
	Bin          string   `yaml:"bin"`
	Build        string   `yaml:"build"`
	SrcSubdir    string   `yaml:"srcSubdir"`
	Asset        string   `yaml:"asset"`
	Skill        string   `yaml:"skill"`
	GoVersionVar string   `yaml:"goVersionVar"`
	Requires     []string `yaml:"requires"`
	Auth         *Auth    `yaml:"auth"`
}

// Manifest is the whole catalog.
type Manifest struct {
	Version int    `yaml:"version"`
	Tools   []Tool `yaml:"tools"`
}

// Load returns the catalog, preferring a fresh remote copy and falling back to
// the copy embedded at build time. A network failure is never fatal: an
// installer that cannot install because GitHub is slow would be a poor trade.
//
// FACILE_CATALOG points at a local file and wins over everything, which is the
// only way to try a catalog edit without publishing it first.
func Load(cachePath string) *Manifest {
	if local := os.Getenv("FACILE_CATALOG"); local != "" {
		if raw, err := os.ReadFile(local); err == nil {
			if m, err := parse(raw); err == nil {
				return m
			}
		}
	}
	if raw, err := os.ReadFile(cachePath); err == nil && fresh(cachePath) {
		if m, err := parse(raw); err == nil {
			return m
		}
	}
	if raw, err := fetch(); err == nil {
		if m, err := parse(raw); err == nil {
			writeCache(cachePath, raw)
			return m
		}
	}
	m, err := parse(embedded)
	if err != nil {
		panic("embedded tools.yml is invalid: " + err.Error())
	}
	return m
}

// Refresh forces a fetch from the remote catalog and updates the cache.
func Refresh(cachePath string) (*Manifest, error) {
	raw, err := fetch()
	if err != nil {
		return nil, err
	}
	m, err := parse(raw)
	if err != nil {
		return nil, err
	}
	writeCache(cachePath, raw)
	return m, nil
}

// Get returns the tool with the given name.
func (m *Manifest) Get(name string) (Tool, bool) {
	for _, t := range m.Tools {
		if strings.EqualFold(t.Name, name) {
			return t, true
		}
	}
	return Tool{}, false
}

// Names returns every tool name in catalog order.
func (m *Manifest) Names() []string {
	names := make([]string, 0, len(m.Tools))
	for _, t := range m.Tools {
		names = append(names, t.Name)
	}
	return names
}

func parse(raw []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if len(m.Tools) == 0 {
		return nil, fmt.Errorf("catalog lists no tools")
	}
	return &m, nil
}

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
	_ = os.WriteFile(path, raw, 0o644)
}
