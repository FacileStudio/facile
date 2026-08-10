package manifest

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A cache written by the previous binary must not survive an upgrade. Serving
// it hides the very change the user just installed.
func TestCacheIsStaleOnceTheBinaryIsNewer(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "tools.yml")
	if err := os.WriteFile(cache, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	self, err := os.Executable()
	if err != nil {
		t.Skip("cannot locate the test binary")
	}
	binary, err := os.Stat(self)
	if err != nil {
		t.Skip("cannot stat the test binary")
	}

	older := binary.ModTime().Add(-time.Hour)
	if err := os.Chtimes(cache, older, older); err != nil {
		t.Fatal(err)
	}
	if fresh(cache) {
		t.Fatal("a cache older than the running binary must be refetched")
	}

	newer := binary.ModTime().Add(time.Minute)
	if err := os.Chtimes(cache, newer, newer); err != nil {
		t.Fatal(err)
	}
	if !fresh(cache) {
		t.Fatal("a cache written after the binary is still fresh")
	}
}

func TestFacileCatalogOverridesEverything(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, "local.yml")
	body := "version: 1\ntools:\n  - name: only\n    bin: only\n"
	if err := os.WriteFile(local, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FACILE_CATALOG", local)

	m := Load(filepath.Join(dir, "cache.yml"))
	if len(m.Tools) != 1 || m.Tools[0].Name != "only" {
		t.Fatalf("the local catalog was ignored: %v", m.Names())
	}
}

// An unreadable override falls through rather than leaving facile with nothing.
func TestAMissingOverrideFallsBackToEmbedded(t *testing.T) {
	t.Setenv("FACILE_CATALOG", filepath.Join(t.TempDir(), "absent.yml"))

	m := Load(filepath.Join(t.TempDir(), "cache.yml"))
	if len(m.Tools) == 0 {
		t.Fatal("expected the embedded catalog")
	}
}
