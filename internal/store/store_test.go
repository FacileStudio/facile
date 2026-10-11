package store

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSourcesPaths_OrderAndFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	paths := SourcesPaths()
	want := []string{
		filepath.Join(home, ".facile.yml"),
		filepath.Join(home, ".facile.yaml"),
		filepath.Join(home, ".config", "facile", "config.yml"),
		filepath.Join(home, ".config", "facile", "config.yaml"),
		filepath.Join(home, ".config", "facile", "sources.yml"),
		filepath.Join(home, ".config", "facile", "sources.yaml"),
	}
	if !slices.Equal(paths, want) {
		t.Errorf("unexpected paths:\ngot:  %v\nwant: %v", paths, want)
	}
}

func TestSourcesPaths_RespectsXDG(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	paths := SourcesPaths()
	wantYml := filepath.Join(xdg, "facile", "config.yml")
	wantYaml := filepath.Join(xdg, "facile", "config.yaml")
	if paths[2] != wantYml || paths[3] != wantYaml {
		t.Errorf("expected XDG paths %q and %q, got %q and %q", wantYml, wantYaml, paths[2], paths[3])
	}
}

func TestSourcesPath_PrefersConfigYmlWhenNoDotfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".config", "facile", "config.yml")
	srcPath := filepath.Join(home, ".config", "facile", "sources.yml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	found := SourcesPath()
	if found != cfgPath {
		t.Errorf("SourcesPath() = %q, want %q", found, cfgPath)
	}
}

func TestSourcesPath_FindsYamlVariant(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgYaml := filepath.Join(home, ".config", "facile", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgYaml), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgYaml, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	found := SourcesPath()
	if found != cfgYaml {
		t.Errorf("SourcesPath() = %q, want %q", found, cfgYaml)
	}
}
