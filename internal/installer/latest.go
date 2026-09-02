package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const latestMaxAge = 24 * time.Hour

// Latest maps each repository to its newest published tag, served from a cache
// that is refreshed at most once a day. `facile list` runs on every shell, and
// resolving ten redirects each time would turn a local listing into a second of
// network for information that moves weekly.
//
// A repository that will not resolve is cached as an empty tag rather than
// dropped, so a tool with no published release does not force a refresh on
// every run. A refresh that yields nothing at all returns the cached copy, even
// stale: a listing that fails because GitHub is unreachable would be a poor
// trade for a marker nobody asked for.
//
// Callers that must be right rather than fast — `update` deciding whether to
// download — call LatestTag directly.
func Latest(cachePath string, repos []string, force bool) map[string]string {
	cached := readLatest(cachePath)
	if !force && freshLatest(cachePath) && covers(cached, repos) {
		return cached
	}
	resolved := resolveAll(repos)
	if len(resolved) == 0 {
		return cached
	}
	merged := make(map[string]string, len(cached)+len(resolved))
	for repo, tag := range cached {
		merged[repo] = tag
	}
	for repo, tag := range resolved {
		merged[repo] = tag
	}
	writeLatest(cachePath, merged)
	return merged
}

func resolveAll(repos []string) map[string]string {
	tags := make([]string, len(repos))
	var wg sync.WaitGroup
	for i, repo := range repos {
		wg.Go(func() {
			if tag, err := LatestTag(repo); err == nil {
				tags[i] = tag
			}
		})
	}
	wg.Wait()

	out := make(map[string]string, len(repos))
	for i, repo := range repos {
		out[repo] = tags[i]
	}
	return out
}

func covers(cached map[string]string, repos []string) bool {
	for _, repo := range repos {
		if _, ok := cached[repo]; !ok {
			return false
		}
	}
	return true
}

// ReadLatest reads the on-disk cache of the latest published tag per repository.
// It is the caller's responsibility to not depend on the returned data being
// current — the cache is advisory, and stale entries are indistinguishable from
// correct ones.
func ReadLatest(path string) map[string]string {
	return readLatest(path)
}

// WriteLatest merges tags into the on-disk cache and writes it back. The merged
// map includes every entry from the existing cache plus the new tags, so a
// partial update never drops entries from a different caller.
func WriteLatest(path string, tags map[string]string) {
	cached := readLatest(path)
	merged := make(map[string]string, len(cached)+len(tags))
	for repo, tag := range cached {
		merged[repo] = tag
	}
	for repo, tag := range tags {
		merged[repo] = tag
	}
	writeLatest(path, merged)
}

func readLatest(path string) map[string]string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var tags map[string]string
	if err := json.Unmarshal(raw, &tags); err != nil {
		return nil
	}
	return tags
}

func writeLatest(path string, tags map[string]string) {
	raw, err := json.Marshal(tags)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	os.WriteFile(path, raw, 0o644)
}

func freshLatest(path string) bool {
	info, err := os.Stat(path)
	return err == nil && time.Since(info.ModTime()) < latestMaxAge
}
