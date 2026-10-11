package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourcesPaths_OrderAndFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	paths := SourcesPaths()
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}
	wantFirst := filepath.Join(home, ".facile.yml")
	wantSecond := filepath.Join(home, ".config", "facile", "config.yml")
	wantThird := filepath.Join(home, ".config", "facile", "sources.yml")
	if paths[0] != wantFirst || paths[1] != wantSecond || paths[2] != wantThird {
		t.Errorf("unexpected paths: got %v", paths)
	}
}

func TestSourcesPaths_RespectsXDG(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	paths := SourcesPaths()
	wantSecond := filepath.Join(xdg, "facile", "config.yml")
	if paths[1] != wantSecond {
		t.Errorf("expected XDG path %q, got %q", wantSecond, paths[1])
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
