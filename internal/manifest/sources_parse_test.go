package manifest

import (
	"testing"
)

func TestParseSources_RootLevelMap(t *testing.T) {
	yaml := `palette: saravenpi/palette
pres: saravenpi/pres
`
	cfg, err := ParseSources([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSources failed: %v", err)
	}
	if len(cfg.Single) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(cfg.Single))
	}
	if cfg.Single[0].Name != "palette" || cfg.Single[0].Repo != "saravenpi/palette" {
		t.Errorf("unexpected first tool: %+v", cfg.Single[0])
	}
	if cfg.Single[1].Name != "pres" || cfg.Single[1].Repo != "saravenpi/pres" {
		t.Errorf("unexpected second tool: %+v", cfg.Single[1])
	}
}

func TestParseSources_BranchPin(t *testing.T) {
	yaml := `palette: saravenpi/palette@v1.2.0
ripgrep: BurntSushi/ripgrep@beta
`
	cfg, err := ParseSources([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSources failed: %v", err)
	}
	if len(cfg.Single) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(cfg.Single))
	}
	if cfg.Single[0].Branch != "v1.2.0" {
		t.Errorf("expected branch v1.2.0, got %q", cfg.Single[0].Branch)
	}
	if cfg.Single[1].Branch != "beta" {
		t.Errorf("expected branch beta, got %q", cfg.Single[1].Branch)
	}
}

func TestParseSources_BareSequence(t *testing.T) {
	yaml := `- saravenpi/palette
- saravenpi/pres@main
`
	cfg, err := ParseSources([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSources failed: %v", err)
	}
	if len(cfg.Single) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(cfg.Single))
	}
	if cfg.Single[0].Name != "palette" || cfg.Single[0].Repo != "saravenpi/palette" {
		t.Errorf("unexpected tool 0: %+v", cfg.Single[0])
	}
	if cfg.Single[1].Name != "pres" || cfg.Single[1].Branch != "main" {
		t.Errorf("unexpected tool 1: %+v", cfg.Single[1])
	}
}

func TestParseSources_ToolsSection(t *testing.T) {
	yaml := `tools:
  palette: saravenpi/palette
lists:
  - https://example.com/repos.txt
`
	cfg, err := ParseSources([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSources failed: %v", err)
	}
	if len(cfg.Single) != 1 || cfg.Single[0].Name != "palette" {
		t.Fatalf("unexpected tools: %+v", cfg.Single)
	}
	if len(cfg.Lists) != 1 || cfg.Lists[0].URL != "https://example.com/repos.txt" {
		t.Fatalf("unexpected lists: %+v", cfg.Lists)
	}
	if cfg.Lists[0].Name != "repos" {
		t.Errorf("expected list name repos, got %q", cfg.Lists[0].Name)
	}
}

func TestParseSources_ExtendedObject(t *testing.T) {
	yaml := `custom:
  repo: myorg/custom
  build: rust
  bin: custom-bin
`
	cfg, err := ParseSources([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSources failed: %v", err)
	}
	if len(cfg.Single) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(cfg.Single))
	}
	s := cfg.Single[0]
	if s.Name != "custom" || s.Repo != "myorg/custom" || s.Build != "rust" || s.Bin != "custom-bin" {
		t.Errorf("unexpected tool fields: %+v", s)
	}
}

func TestParseSources_LegacyFormat(t *testing.T) {
	yaml := `single:
  - name: mytool
    repo: me/mytool
lists:
  - name: community
    url: https://example.com/list
`
	cfg, err := ParseSources([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSources failed: %v", err)
	}
	if len(cfg.Single) != 1 || cfg.Single[0].Name != "mytool" {
		t.Fatalf("unexpected single: %+v", cfg.Single)
	}
	if len(cfg.Lists) != 1 || cfg.Lists[0].Name != "community" {
		t.Fatalf("unexpected lists: %+v", cfg.Lists)
	}
}
