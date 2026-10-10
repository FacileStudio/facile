package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegrationSourcesFileLoading(t *testing.T) {
	t.Run("LoadSources parses sources.yml", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		sourcesPath := filepath.Join(home, ".facile.yml")
		content := `single:
  - name: mytool
    repo: me/mytool
    bin: mytool
    build: go
lists:
  - name: community
    url: https://example.com/list
`
		if err := os.WriteFile(sourcesPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := LoadSources()
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Single) != 1 {
			t.Fatalf("expected 1 single source, got %d", len(cfg.Single))
		}
		if cfg.Single[0].Name != "mytool" {
			t.Errorf("name = %q, want mytool", cfg.Single[0].Name)
		}
		if cfg.Single[0].Repo != "me/mytool" {
			t.Errorf("repo = %q, want me/mytool", cfg.Single[0].Repo)
		}
		if len(cfg.Lists) != 1 {
			t.Fatalf("expected 1 list source, got %d", len(cfg.Lists))
		}
		if cfg.Lists[0].Name != "community" {
			t.Errorf("list name = %q, want community", cfg.Lists[0].Name)
		}
	})

	t.Run("SourcesPath returns priority path", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		homeFile := filepath.Join(home, ".facile.yml")
		xdgFile := filepath.Join(home, ".config", "facile", "sources.yml")

		if err := os.MkdirAll(filepath.Dir(xdgFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(xdgFile, []byte("single: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(homeFile, []byte("single: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		path := SourcesPath()
		if path != homeFile {
			t.Errorf("SourcesPath = %q, want %q (home file has priority)", path, homeFile)
		}
	})

	t.Run("LoadSources returns empty config when no file", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		cfg, err := LoadSources()
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Single) != 0 || len(cfg.Lists) != 0 {
			t.Errorf("expected empty config, got singles=%d lists=%d", len(cfg.Single), len(cfg.Lists))
		}
	})

	t.Run("LoadSources returns empty on missing home", func(t *testing.T) {
		t.Setenv("HOME", "/nonexistent/deadbeef12345")

		cfg, err := LoadSources()
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Single) != 0 || len(cfg.Lists) != 0 {
			t.Errorf("expected empty config, got singles=%d lists=%d", len(cfg.Single), len(cfg.Lists))
		}
	})
}

func TestIntegrationCatalogMergingWithSingleSources(t *testing.T) {
	t.Run("single sources shadow base and append new", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		sourcesPath := filepath.Join(home, ".facile.yml")
		sourcesContent := `single:
  - name: sablier
    repo: me/my-sablier-fork
    bin: sablier
    build: rust
    summary: My custom sablier
  - name: mytool
    repo: me/mytool
    bin: mytool
    build: go
`
		if err := os.WriteFile(sourcesPath, []byte(sourcesContent), 0o644); err != nil {
			t.Fatal(err)
		}

		cachePath := filepath.Join(t.TempDir(), "cache.yml")
		m, err := Load(cachePath)
		if err != nil {
			t.Fatal(err)
		}

		sablier, ok := m.Get("sablier")
		if !ok {
			t.Fatal("sablier not found in merged catalog")
		}
		if sablier.Repo != "me/my-sablier-fork" {
			t.Errorf("sablier repo = %q, want me/my-sablier-fork (should be shadowed)", sablier.Repo)
		}
		if sablier.Summary != "My custom sablier" {
			t.Errorf("sablier summary = %q, want 'My custom sablier'", sablier.Summary)
		}

		mytool, ok := m.Get("mytool")
		if !ok {
			t.Fatal("mytool not found in merged catalog")
		}
		if mytool.Repo != "me/mytool" {
			t.Errorf("mytool repo = %q, want me/mytool", mytool.Repo)
		}

		opus, ok := m.Get("opus")
		if !ok {
			t.Fatal("opus not found — non-shadowed tools should be preserved")
		}
		if opus.Repo != "FacileStudio/opus-cli" {
			t.Errorf("opus repo = %q, want FacileStudio/opus-cli", opus.Repo)
		}
	})

	t.Run("LoadBase does not include user sources", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		sourcesPath := filepath.Join(home, ".facile.yml")
		sourcesContent := `single:
  - name: mytool
    repo: me/mytool
    bin: mytool
`
		if err := os.WriteFile(sourcesPath, []byte(sourcesContent), 0o644); err != nil {
			t.Fatal(err)
		}

		cachePath := filepath.Join(t.TempDir(), "cache.yml")
		m, err := LoadBase(cachePath)
		if err != nil {
			t.Fatal(err)
		}

		_, ok := m.Get("mytool")
		if ok {
			t.Error("LoadBase should not include user sources")
		}
	})

	t.Run("Load returns merged catalog", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		sourcesPath := filepath.Join(home, ".facile.yml")
		sourcesContent := `single:
  - name: mytool
    repo: me/mytool
    bin: mytool
`
		if err := os.WriteFile(sourcesPath, []byte(sourcesContent), 0o644); err != nil {
			t.Fatal(err)
		}

		cachePath := filepath.Join(t.TempDir(), "cache.yml")
		m, err := Load(cachePath)
		if err != nil {
			t.Fatal(err)
		}

		_, ok := m.Get("mytool")
		if !ok {
			t.Error("Load should include user sources")
		}

		_, ok = m.Get("opus")
		if !ok {
			t.Error("Load should preserve base tools")
		}
	})
}

func TestIntegrationListSourceParsing(t *testing.T) {
	t.Run("plain text format", func(t *testing.T) {
		body := `# community tools
owner/repo1
owner/repo2

# another section
owner/repo3`
		repos := ParseListSource(body)
		if len(repos) != 3 {
			t.Fatalf("expected 3 repos, got %d: %v", len(repos), repos)
		}
		expected := []string{"owner/repo1", "owner/repo2", "owner/repo3"}
		for i, want := range expected {
			if repos[i] != want {
				t.Errorf("repos[%d] = %q, want %q", i, repos[i], want)
			}
		}
	})

	t.Run("PEP 503 HTML format", func(t *testing.T) {
		body := `<!DOCTYPE html>
<html>
<body>
  <a href="/owner/repo1/">repo1</a>
  <a href="/owner/repo2/">repo2</a>
</body>
</html>`
		repos := ParseListSource(body)
		if len(repos) != 2 {
			t.Fatalf("expected 2 repos, got %d: %v", len(repos), repos)
		}
		if repos[0] != "owner/repo1" {
			t.Errorf("repos[0] = %q, want owner/repo1", repos[0])
		}
		if repos[1] != "owner/repo2" {
			t.Errorf("repos[1] = %q, want owner/repo2", repos[1])
		}
	})

	t.Run("deduplication", func(t *testing.T) {
		body := `owner/repo1
owner/repo2
owner/repo1
owner/repo2
owner/repo3`
		repos := ParseListSource(body)
		if len(repos) != 3 {
			t.Fatalf("expected 3 unique repos, got %d: %v", len(repos), repos)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		repos := ParseListSource("")
		if len(repos) != 0 {
			t.Fatalf("expected 0 repos, got %d", len(repos))
		}
	})

	t.Run("only comments and empty lines", func(t *testing.T) {
		body := `
# comment
  # indented comment

`
		repos := ParseListSource(body)
		if len(repos) != 0 {
			t.Fatalf("expected 0 repos, got %d", len(repos))
		}
	})

	t.Run("PEP 503 with deduplication", func(t *testing.T) {
		body := `<a href="/a/b/"></a>
<a href="/c/d/"></a>
<a href="/a/b/"></a>`
		repos := ParseListSource(body)
		if len(repos) != 2 {
			t.Fatalf("expected 2 unique repos, got %d: %v", len(repos), repos)
		}
	})

	t.Run("plain text ignores lines without slash", func(t *testing.T) {
		body := `owner/repo
not-a-repo
another/valid/repo
singleword
owner/ok`
		repos := ParseListSource(body)
		if len(repos) != 2 {
			t.Fatalf("expected 2 repos (exactly one slash), got %d: %v", len(repos), repos)
		}
		if repos[0] != "owner/repo" || repos[1] != "owner/ok" {
			t.Errorf("got %v", repos)
		}
	})
}

func TestIntegrationFacileTomlParsing(t *testing.T) {
	t.Run("valid facile.toml", func(t *testing.T) {
		raw := `name = "mycli"
repo = "me/mycli"
summary = "A custom CLI"
branch = "main"
bin = "mycli"
build = "rust"
versionCmd = "mycli --version"
versionPattern = "v?(\\d+\\.\\d+\\.\\d+)"
`
		ft, err := parseFacileToml([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if ft.Name != "mycli" {
			t.Errorf("name = %q, want mycli", ft.Name)
		}
		if ft.Repo != "me/mycli" {
			t.Errorf("repo = %q, want me/mycli", ft.Repo)
		}
		if ft.Summary != "A custom CLI" {
			t.Errorf("summary = %q", ft.Summary)
		}
		if ft.VersionCmd != "mycli --version" {
			t.Errorf("versionCmd = %q", ft.VersionCmd)
		}
	})

	t.Run("facile.toml missing name", func(t *testing.T) {
		raw := `repo = "me/mycli"
`
		_, err := parseFacileToml([]byte(raw))
		if err == nil {
			t.Fatal("expected error for missing name")
		}
		if !strings.Contains(err.Error(), "name is required") {
			t.Errorf("error = %v, want 'name is required'", err)
		}
	})

	t.Run("facile.toml missing repo", func(t *testing.T) {
		raw := `name = "mycli"
`
		_, err := parseFacileToml([]byte(raw))
		if err == nil {
			t.Fatal("expected error for missing repo")
		}
		if !strings.Contains(err.Error(), "repo is required") {
			t.Errorf("error = %v, want 'repo is required'", err)
		}
	})

	t.Run("facile.toml invalid versionPattern", func(t *testing.T) {
		raw := `name = "mycli"
repo = "me/mycli"
versionPattern = "[bad"
`
		_, err := parseFacileToml([]byte(raw))
		if err == nil {
			t.Fatal("expected error for invalid versionPattern")
		}
		if !strings.Contains(err.Error(), "invalid versionPattern") {
			t.Errorf("error = %v, want 'invalid versionPattern'", err)
		}
	})

	t.Run("facile.toml ToTool with defaults", func(t *testing.T) {
		ft := &FacileToml{
			Name: "mycli",
			Repo: "me/mycli",
		}
		tool, err := ft.ToTool()
		if err != nil {
			t.Fatal(err)
		}
		if tool.Branch != "main" {
			t.Errorf("branch = %q, want main", tool.Branch)
		}
		if tool.Build != "go" {
			t.Errorf("build = %q, want go", tool.Build)
		}
		if tool.Bin != "mycli" {
			t.Errorf("bin = %q, want mycli", tool.Bin)
		}
		if tool.SrcSubdir != "." {
			t.Errorf("srcSubdir = %q, want .", tool.SrcSubdir)
		}
	})

	t.Run("facile.toml ToTool with explicit values preserved", func(t *testing.T) {
		ft := &FacileToml{
			Name:           "mycli",
			Repo:           "me/mycli",
			Branch:         "develop",
			Build:          "rust",
			Bin:            "mc",
			SrcSubdir:      "src",
			Skill:          "myskill",
			VersionCmd:     "mc version",
			VersionPattern: "v(\\d+\\.\\d+\\.\\d+)",
		}
		tool, err := ft.ToTool()
		if err != nil {
			t.Fatal(err)
		}
		if tool.Branch != "develop" {
			t.Errorf("branch = %q", tool.Branch)
		}
		if tool.Build != "rust" {
			t.Errorf("build = %q", tool.Build)
		}
		if tool.Bin != "mc" {
			t.Errorf("bin = %q", tool.Bin)
		}
		if tool.SrcSubdir != "src" {
			t.Errorf("srcSubdir = %q", tool.SrcSubdir)
		}
		if tool.Cmd != "mc version" {
			t.Errorf("versionCmd = %q", tool.Cmd)
		}
	})

	t.Run("facile.toml ToTool with invalid versionPattern", func(t *testing.T) {
		ft := &FacileToml{
			Name:           "mycli",
			Repo:           "me/mycli",
			VersionPattern: "[",
		}
		_, err := ft.ToTool()
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "invalid versionPattern") {
			t.Errorf("error = %v", err)
		}
	})
}

func TestIntegrationMiseTomlParsing(t *testing.T) {
	t.Run("valid mise.toml with [facile] block", func(t *testing.T) {
		raw := `[tools]
node = "20"

[facile]
name = "mycli"
repo = "me/mycli"
summary = "A custom CLI"
branch = "main"
`
		ft, err := parseMiseTomlFacade([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if ft.Name != "mycli" {
			t.Errorf("name = %q, want mycli", ft.Name)
		}
		if ft.Repo != "me/mycli" {
			t.Errorf("repo = %q, want me/mycli", ft.Repo)
		}
	})

	t.Run("mise.toml missing [facile] block", func(t *testing.T) {
		raw := `[tools]
node = "20"
`
		_, err := parseMiseTomlFacade([]byte(raw))
		if err == nil {
			t.Fatal("expected error for missing [facile]")
		}
		if !errors.Is(err, ErrNoFacileToml) {
			t.Errorf("error = %v, want ErrNoFacileToml", err)
		}
	})

	t.Run("mise.toml [facile] missing name", func(t *testing.T) {
		raw := `[facile]
repo = "me/mycli"
`
		_, err := parseMiseTomlFacade([]byte(raw))
		if err == nil {
			t.Fatal("expected error for missing name")
		}
		if !strings.Contains(err.Error(), "name is required") {
			t.Errorf("error = %v", err)
		}
	})

	t.Run("mise.toml [facile] missing repo", func(t *testing.T) {
		raw := `[facile]
name = "mycli"
`
		_, err := parseMiseTomlFacade([]byte(raw))
		if err == nil {
			t.Fatal("expected error for missing repo")
		}
		if !strings.Contains(err.Error(), "repo is required") {
			t.Errorf("error = %v", err)
		}
	})

	t.Run("mise.toml [facile] ToTool defaults", func(t *testing.T) {
		ft := &FacileToml{
			Name: "mycli",
			Repo: "me/mycli",
		}
		tool, err := ft.ToTool()
		if err != nil {
			t.Fatal(err)
		}
		if tool.Branch != "main" {
			t.Errorf("branch = %q, want main", tool.Branch)
		}
		if tool.Build != "go" {
			t.Errorf("build = %q, want go", tool.Build)
		}
		if tool.Bin != "mycli" {
			t.Errorf("bin = %q, want mycli", tool.Bin)
		}
	})
}

func TestIntegrationMerge(t *testing.T) {
	t.Run("shadow existing and append new", func(t *testing.T) {
		base := &Manifest{
			Version: 1,
			Tools: []Tool{
				{Name: "A", Repo: "base/A"},
				{Name: "B", Repo: "base/B"},
				{Name: "C", Repo: "base/C"},
			},
		}
		layer := &Manifest{
			Tools: []Tool{
				{Name: "B", Repo: "layer/B-shadow"},
				{Name: "D", Repo: "layer/D"},
			},
		}
		merged := Merge(base, layer)
		if len(merged.Tools) != 4 {
			t.Fatalf("expected 4 tools, got %d", len(merged.Tools))
		}
		if merged.Tools[0].Name != "A" {
			t.Errorf("tools[0] = %s, want A", merged.Tools[0].Name)
		}
		if merged.Tools[1].Name != "B" || merged.Tools[1].Repo != "layer/B-shadow" {
			t.Errorf("tools[1] = %s (%s), want B (layer/B-shadow)", merged.Tools[1].Name, merged.Tools[1].Repo)
		}
		if merged.Tools[2].Name != "C" {
			t.Errorf("tools[2] = %s, want C", merged.Tools[2].Name)
		}
		if merged.Tools[3].Name != "D" || merged.Tools[3].Repo != "layer/D" {
			t.Errorf("tools[3] = %s (%s), want D (layer/D)", merged.Tools[3].Name, merged.Tools[3].Repo)
		}
	})

	t.Run("nil base returns nil", func(t *testing.T) {
		merged := Merge(nil)
		if merged != nil {
			t.Fatal("expected nil")
		}
	})

	t.Run("nil layers are skipped", func(t *testing.T) {
		base := &Manifest{
			Tools: []Tool{{Name: "A"}},
		}
		merged := Merge(base, nil, nil)
		if len(merged.Tools) != 1 || merged.Tools[0].Name != "A" {
			t.Fatalf("tools = %v", merged.Names())
		}
	})

	t.Run("case-insensitive shadowing", func(t *testing.T) {
		base := &Manifest{
			Tools: []Tool{
				{Name: "MyTool", Repo: "base/mytool"},
				{Name: "Other", Repo: "base/other"},
			},
		}
		layer := &Manifest{
			Tools: []Tool{
				{Name: "mytool", Repo: "layer/mytool"},
			},
		}
		merged := Merge(base, layer)
		if len(merged.Tools) != 2 {
			t.Fatalf("expected 2 tools, got %d", len(merged.Tools))
		}
		if merged.Tools[0].Repo != "layer/mytool" {
			t.Errorf("tools[0].Repo = %q, want layer/mytool", merged.Tools[0].Repo)
		}
	})

	t.Run("multiple layers merge in order", func(t *testing.T) {
		base := &Manifest{Tools: []Tool{{Name: "A"}, {Name: "B"}}}
		layer1 := &Manifest{Tools: []Tool{{Name: "C"}}}
		layer2 := &Manifest{Tools: []Tool{{Name: "B", Repo: "layer2/B"}, {Name: "D"}}}
		merged := Merge(base, layer1, layer2)
		if len(merged.Tools) != 4 {
			t.Fatalf("expected 4 tools, got %d", len(merged.Tools))
		}
		names := merged.Names()
		expected := []string{"A", "B", "C", "D"}
		for i, want := range expected {
			if names[i] != want {
				t.Errorf("tools[%d] = %s, want %s", i, names[i], want)
			}
		}
		if merged.Tools[1].Repo != "layer2/B" {
			t.Errorf("B should be from layer2, got repo=%q", merged.Tools[1].Repo)
		}
	})

	t.Run("empty layers", func(t *testing.T) {
		base := &Manifest{Tools: []Tool{{Name: "A"}}}
		merged := Merge(base)
		if len(merged.Tools) != 1 || merged.Tools[0].Name != "A" {
			t.Fatal("expected base unchanged")
		}
	})
}

func TestIntegrationVersionPatternMatching(t *testing.T) {
	t.Run("simple semver pattern", func(t *testing.T) {
		output := "mycli version 1.2.3"
		pattern := `(\d+\.\d+\.\d+)`
		v := MatchVersion(output, pattern)
		if v != "1.2.3" {
			t.Errorf("version = %q, want 1.2.3", v)
		}
	})

	t.Run("pattern with v prefix capture", func(t *testing.T) {
		output := "v2.0.1"
		pattern := `v(\d+\.\d+\.\d+)`
		v := MatchVersion(output, pattern)
		if v != "2.0.1" {
			t.Errorf("version = %q, want 2.0.1", v)
		}
	})

	t.Run("no capture group returns full match", func(t *testing.T) {
		output := "1.2.3"
		pattern := `\d+\.\d+\.\d+`
		v := MatchVersion(output, pattern)
		if v != "1.2.3" {
			t.Errorf("version = %q, want 1.2.3", v)
		}
	})

	t.Run("non-matching pattern returns empty", func(t *testing.T) {
		output := "foo"
		pattern := `(\d+\.\d+\.\d+)`
		v := MatchVersion(output, pattern)
		if v != "" {
			t.Errorf("version = %q, want empty", v)
		}
	})

	t.Run("invalid pattern returns empty", func(t *testing.T) {
		output := "1.2.3"
		v := MatchVersion(output, "[")
		if v != "" {
			t.Errorf("version = %q, want empty", v)
		}
	})

	t.Run("empty output returns empty", func(t *testing.T) {
		v := MatchVersion("", `(\d+\.\d+\.\d+)`)
		if v != "" {
			t.Errorf("version = %q, want empty", v)
		}
	})

	t.Run("empty pattern returns empty", func(t *testing.T) {
		v := MatchVersion("1.2.3", "")
		if v != "" {
			t.Errorf("version = %q, want empty", v)
		}
	})

	t.Run("complex output with capture", func(t *testing.T) {
		output := "mycli 3.4.5 (abc123) linux/amd64"
		pattern := `mycli (\d+\.\d+\.\d+)`
		v := MatchVersion(output, pattern)
		if v != "3.4.5" {
			t.Errorf("version = %q, want 3.4.5", v)
		}
	})

	t.Run("multiple capture groups return first", func(t *testing.T) {
		output := "1.2.3-4-gabc123"
		pattern := `(\d+\.\d+\.\d+)-(\d+)`
		v := MatchVersion(output, pattern)
		if v != "1.2.3" {
			t.Errorf("version = %q, want 1.2.3", v)
		}
	})
}

func TestIntegrationSingleSourceToTool(t *testing.T) {
	t.Run("defaults are filled", func(t *testing.T) {
		s := &SingleSource{
			Name: "mycli",
			Repo: "me/mycli",
		}
		tool, err := s.ToTool()
		if err != nil {
			t.Fatal(err)
		}
		if tool.Branch != "main" {
			t.Errorf("branch = %q, want main", tool.Branch)
		}
		if tool.Build != "go" {
			t.Errorf("build = %q, want go", tool.Build)
		}
		if tool.Bin != "mycli" {
			t.Errorf("bin = %q, want mycli", tool.Bin)
		}
		if tool.SrcSubdir != "." {
			t.Errorf("srcSubdir = %q, want .", tool.SrcSubdir)
		}
	})

	t.Run("explicit values are preserved", func(t *testing.T) {
		s := &SingleSource{
			Name:      "mycli",
			Repo:      "me/mycli",
			Branch:    "develop",
			Build:     "rust",
			Bin:       "mc",
			SrcSubdir: "src",
		}
		tool, err := s.ToTool()
		if err != nil {
			t.Fatal(err)
		}
		if tool.Branch != "develop" {
			t.Errorf("branch = %q", tool.Branch)
		}
		if tool.Build != "rust" {
			t.Errorf("build = %q", tool.Build)
		}
		if tool.Bin != "mc" {
			t.Errorf("bin = %q", tool.Bin)
		}
		if tool.SrcSubdir != "src" {
			t.Errorf("srcSubdir = %q", tool.SrcSubdir)
		}
	})

	t.Run("empty name returns error", func(t *testing.T) {
		s := &SingleSource{
			Repo: "me/mycli",
		}
		_, err := s.ToTool()
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "missing name") {
			t.Errorf("error = %v", err)
		}
	})

	t.Run("invalid versionPattern returns error", func(t *testing.T) {
		s := &SingleSource{
			Name: "mycli",
			Repo: "me/mycli",
			VersionDetection: VersionDetection{
				Pattern: "[",
			},
		}
		_, err := s.ToTool()
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "invalid versionPattern") {
			t.Errorf("error = %v", err)
		}
	})

	t.Run("versionDetection fields are copied", func(t *testing.T) {
		s := &SingleSource{
			Name: "mycli",
			Repo: "me/mycli",
			VersionDetection: VersionDetection{
				Cmd:     "mycli --version",
				Pattern: `v(\d+\.\d+\.\d+)`,
			},
		}
		tool, err := s.ToTool()
		if err != nil {
			t.Fatal(err)
		}
		if tool.Cmd != "mycli --version" {
			t.Errorf("versionCmd = %q, want mycli --version", tool.Cmd)
		}
		if tool.Pattern != `v(\d+\.\d+\.\d+)` {
			t.Errorf("versionPattern = %q", tool.Pattern)
		}
	})
}

func TestIntegrationLoadBaseVsLoad(t *testing.T) {
	t.Run("Load includes user sources", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		sourcesPath := filepath.Join(home, ".facile.yml")
		sourcesContent := `single:
  - name: integration-test-tool
    repo: me/integration-test
    bin: itt
`
		if err := os.WriteFile(sourcesPath, []byte(sourcesContent), 0o644); err != nil {
			t.Fatal(err)
		}

		cachePath := filepath.Join(t.TempDir(), "cache.yml")
		m, err := Load(cachePath)
		if err != nil {
			t.Fatal(err)
		}
		_, ok := m.Get("integration-test-tool")
		if !ok {
			t.Error("Load should include user source 'integration-test-tool'")
		}
	})

	t.Run("LoadBase excludes user sources", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		sourcesPath := filepath.Join(home, ".facile.yml")
		sourcesContent := `single:
  - name: integration-test-tool
    repo: me/integration-test
    bin: itt
`
		if err := os.WriteFile(sourcesPath, []byte(sourcesContent), 0o644); err != nil {
			t.Fatal(err)
		}

		cachePath := filepath.Join(t.TempDir(), "cache.yml")
		m, err := LoadBase(cachePath)
		if err != nil {
			t.Fatal(err)
		}
		_, ok := m.Get("integration-test-tool")
		if ok {
			t.Error("LoadBase should exclude user source 'integration-test-tool'")
		}
	})
}