package manifest

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed tools.yml
var embedded []byte

// LoadBase returns the catalog without user sources merged. It mirrors Load's
// priority chain (local override → cache → fetch → embedded) but skips the
// merge step so callers can compare user sources against the authoritative
// catalog.
func LoadBase(cachePath string) (*Manifest, error) {
	if m, err := localCatalog(); err == nil && m != nil {
		return m, nil
	}
	if raw, err := os.ReadFile(cachePath); err == nil && fresh(cachePath) {
		if m, err := parse(raw); err == nil {
			return m, nil
		}
	}
	if raw, err := fetch(); err == nil {
		if m, err := parse(raw); err == nil {
			writeCache(cachePath, raw)
			return m, nil
		}
	}
	return embeddedManifest()
}

// Load returns the catalog, merging layers in priority order:
// embedded → remote (cached) → user sources (single + list-discovered).
// A network failure is never fatal: an installer that cannot install
// because GitHub is slow would be a poor trade.
//
// FACILE_CATALOG points at a local file and wins over everything,
// which is the only way to try a catalog edit without publishing it
// first. When set, the local file is the entire catalog — no merge,
// no user sources, no embedded fallback.
func Load(cachePath string) (*Manifest, error) {
	if m, err := localCatalog(); err == nil && m != nil {
		return m, nil
	}

	if raw, err := os.ReadFile(cachePath); err == nil && fresh(cachePath) {
		if m, err := parse(raw); err == nil {
			return mergeCatalogs(m), nil
		}
	}

	if raw, err := fetch(); err == nil {
		if m, err := parse(raw); err == nil {
			writeCache(cachePath, raw)
			return mergeCatalogs(m), nil
		}
	}

	embedded, err := embeddedManifest()
	if err != nil {
		return nil, err
	}
	return mergeCatalogs(embedded), nil
}

// Refresh forces a fetch from the remote catalog, updates the cache,
// and returns the merged manifest including user sources.
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
	return mergeCatalogs(m), nil
}

// localCatalog is the escape hatch for trying a catalog edit before
// publishing it. FACILE_CATALOG wins over everything else, remote
// and embedded alike. An absent or unreadable override is not an
// error: Load falls through to the next source rather than dying
// on an env var that names a stray path.
func localCatalog() (*Manifest, error) {
	local := os.Getenv("FACILE_CATALOG")
	raw, ok := readLocal(local)
	if !ok {
		return nil, nil
	}
	return parse(raw)
}

// readLocal reads one catalog override, reporting not-found as the
// absence of one rather than as an error, so Load can fall through
// to the next source.
func readLocal(local string) ([]byte, bool) {
	if local == "" {
		return nil, false
	}
	raw, err := os.ReadFile(local)
	if err != nil {
		return nil, false
	}
	return raw, true
}

// embeddedManifest is the last resort: the catalog compiled into
// the binary. It can only fail if the build shipped a broken file,
// which is then the reporting error rather than a crash.
func embeddedManifest() (*Manifest, error) {
	m, err := parse(embedded)
	if err != nil {
		return nil, fmt.Errorf("the embedded catalog is invalid — reinstall facile: %s", err.Error())
	}
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
