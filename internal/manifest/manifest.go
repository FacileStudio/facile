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

// Manifest is the whole catalog. MCP names the tools that register their MCP
// server in agent harnesses on install, through each one's own install
// subcommand; it is a list off Tool because Tool sits at the 12-field cap.
type Manifest struct {
	Version int      `yaml:"version"`
	Tools   []Tool   `yaml:"tools"`
	MCP     []string `yaml:"mcp"`
}

// Load returns the catalog, preferring a fresh remote copy and falling back to
// the copy embedded at build time. A network failure is never fatal: an
// installer that cannot install because GitHub is slow would be a poor trade.
//
// FACILE_CATALOG points at a local file and wins over everything, which is the
// only way to try a catalog edit without publishing it first.
func Load(cachePath string) (*Manifest, error) {
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

// localCatalog is the escape hatch for trying a catalog edit before publishing
// it. FACILE_CATALOG wins over everything else, remote and embedded alike. An
// absent or unreadable override is not an error: Load falls through to the next
// source rather than dying on an env var that names a stray path.
func localCatalog() (*Manifest, error) {
	local := os.Getenv("FACILE_CATALOG")
	raw, ok := readLocal(local)
	if !ok {
		return nil, nil
	}
	return parse(raw)
}

// readLocal reads one catalog override, reporting not-found as the absence of
// one rather than as an error, so Load can fall through to the next source.
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

// embeddedManifest is the last resort: the catalog compiled into the binary.
// It can only fail if the build shipped a broken file, which is then the
// reporting error rather than a crash.
func embeddedManifest() (*Manifest, error) {
	m, err := parse(embedded)
	if err != nil {
		return nil, fmt.Errorf("the embedded catalog is invalid — reinstall facile: %s", err.Error())
	}
	return m, nil
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
