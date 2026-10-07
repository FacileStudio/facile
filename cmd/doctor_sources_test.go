package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
)

func TestCheckSourceShadowing_DetectsCollision(t *testing.T) {
	catalog := `version: 1
tools:
  - name: sablier
    bin: sablier
    repo: FacileStudio/sablier
`
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "tools.yml")
	os.WriteFile(catalogPath, []byte(catalog), 0644)
	t.Setenv("FACILE_CATALOG", catalogPath)

	cfg := manifest.SourceConfig{
		Single: []manifest.SingleSource{{Name: "sablier", Repo: "someone/sablier"}},
	}

	base, err := manifest.LoadBase(store.CatalogPath())
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, tl := range base.Tools {
		if tl.Name == "sablier" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected sablier in base catalog")
	}

	catalogNames := make(map[string]bool)
	for _, tl := range base.Tools {
		catalogNames[tl.Name] = true
	}
	if !catalogNames[cfg.Single[0].Name] {
		t.Fatal("shadowing not detected")
	}
}

func TestCheckSourceShadowing_NoFalsePositive(t *testing.T) {
	catalog := `version: 1
tools:
  - name: sablier
    bin: sablier
    repo: FacileStudio/sablier
`
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "tools.yml")
	os.WriteFile(catalogPath, []byte(catalog), 0644)
	t.Setenv("FACILE_CATALOG", catalogPath)

	cfg := manifest.SourceConfig{
		Single: []manifest.SingleSource{{Name: "my-unique-tool", Repo: "someone/tool"}},
	}

	base, err := manifest.LoadBase(store.CatalogPath())
	if err != nil {
		t.Fatal(err)
	}

	catalogNames := make(map[string]bool)
	for _, tl := range base.Tools {
		catalogNames[tl.Name] = true
	}
	if catalogNames[cfg.Single[0].Name] {
		t.Fatal("false positive: my-unique-tool should not collide")
	}
}

func TestCheckSourceShadowing_CaseInsensitive(t *testing.T) {
	catalog := `version: 1
tools:
  - name: Sablier
    bin: sablier
    repo: FacileStudio/sablier
`
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "tools.yml")
	os.WriteFile(catalogPath, []byte(catalog), 0644)
	t.Setenv("FACILE_CATALOG", catalogPath)

	cfg := manifest.SourceConfig{
		Single: []manifest.SingleSource{{Name: "sablier", Repo: "someone/sablier"}},
	}

	base, err := manifest.LoadBase(store.CatalogPath())
	if err != nil {
		t.Fatal(err)
	}

	catalogNames := make(map[string]bool)
	for _, tl := range base.Tools {
		catalogNames[strings.ToLower(tl.Name)] = true
	}
	if !catalogNames[strings.ToLower(cfg.Single[0].Name)] {
		t.Fatal("case-insensitive shadowing not detected")
	}
}