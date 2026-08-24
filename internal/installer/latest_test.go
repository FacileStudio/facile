package installer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLatestServesAFreshCacheWithoutTheNetwork(t *testing.T) {
	path := filepath.Join(t.TempDir(), "latest.json")
	writeLatest(path, map[string]string{"FacileStudio/filet": "v1.2.3"})

	got := Latest(path, []string{"FacileStudio/filet"}, false)

	if got["FacileStudio/filet"] != "v1.2.3" {
		t.Fatalf("cached tag not served: %v", got)
	}
}

func TestLatestKeepsAStaleCacheWhenNothingResolves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "latest.json")
	writeLatest(path, map[string]string{"FacileStudio/filet": "v1.2.3"})
	old := time.Now().Add(-2 * latestMaxAge)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	got := Latest(path, nil, false)

	if got["FacileStudio/filet"] != "v1.2.3" {
		t.Fatalf("stale cache dropped instead of reused: %v", got)
	}
}

func TestCoversRequiresEveryRequestedRepo(t *testing.T) {
	cached := map[string]string{"a": "v1", "b": ""}

	if !covers(cached, []string{"a", "b"}) {
		t.Error("an empty tag must count as cached, or a tool with no release refreshes every run")
	}
	if covers(cached, []string{"a", "c"}) {
		t.Error("a missing repo must force a refresh")
	}
}
