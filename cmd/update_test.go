package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/FacileStudio/facile/internal/manifest"
)

// `facile update` used to redownload every installed tool on every run, so the
// case that matters is the one that skips: it must fire on an exact match and
// on nothing else.
func TestUpToDateSkipsOnlyAnExactVersionMatch(t *testing.T) {
	tool := manifest.Tool{Name: "opus", Bin: "opus", Asset: "opus", Repo: "FacileStudio/opus"}

	cases := []struct {
		name string
		have string
		tag  string
		err  error
		want bool
	}{
		{"installed version matches the latest tag", "opus 0.1.0", "v0.1.0", nil, true},
		{"a tag with no v prefix still matches", "opus 0.1.0", "0.1.0", nil, true},
		{"an older binary is not up to date", "opus 0.1.0", "v0.2.0", nil, false},
		{"a newer binary is not up to date either", "opus 0.3.0", "v0.2.0", nil, false},
		{"an unreadable tag reinstalls rather than guesses", "opus 0.1.0", "", fmt.Errorf("404"), false},
		{"a tool that is not installed is not up to date", "", "v0.1.0", nil, false},
		{"a version line facile cannot parse reinstalls", "opus version 0.1.0", "v0.1.0", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubLatestTag(t, func(string) (string, error) { return tc.tag, tc.err })
			if got := upToDate(tc.have, tool); got != tc.want {
				t.Fatalf("upToDate(%q, tag %q) = %v, want %v", tc.have, tc.tag, got, tc.want)
			}
		})
	}
}

// A tool with no release archive only ever installs from source, where there is
// no tag to compare against.
func TestUpToDateNeverSkipsASourceOnlyTool(t *testing.T) {
	stubLatestTag(t, func(string) (string, error) { return "v0.1.0", nil })

	tool := manifest.Tool{Name: "opus", Bin: "opus", Repo: "FacileStudio/opus"}
	if upToDate("opus 0.1.0", tool) {
		t.Fatal("a tool with no asset has no release to compare against")
	}
}

// stale runs its checks concurrently, so the thing to pin is that every result
// lands against the tool it belongs to. A swapped index reads as the wrong tool
// being skipped, which on a real run is a tool that never updates again.
func TestStaleKeepsTheOutdatedToolsInCatalogOrder(t *testing.T) {
	dir := t.TempDir()
	stubBinDir(t, dir)
	stubLatestTag(t, func(repo string) (string, error) {
		return map[string]string{
			"FacileStudio/opus":    "v0.1.0",
			"FacileStudio/Sablier": "v0.2.0",
			"FacileStudio/Nuage":   "v0.3.0",
			"FacileStudio/Spore":   "v0.6.1",
		}[repo], nil
	})
	stubBinary(t, dir, "opus", "opus 0.1.0")
	stubBinary(t, dir, "sablier", "sablier 0.1.1")
	stubBinary(t, dir, "spore", "spore 0.6.1")

	tools := []manifest.Tool{
		{Name: "opus", Bin: "opus", Asset: "opus", Repo: "FacileStudio/opus"},
		{Name: "sablier", Bin: "sablier", Asset: "sablier", Repo: "FacileStudio/Sablier"},
		{Name: "nuage", Bin: "nuage", Asset: "nuage", Repo: "FacileStudio/Nuage"},
		{Name: "spore", Bin: "spore", Asset: "spore", Repo: "FacileStudio/Spore"},
	}

	var got []string
	for _, tool := range stale(tools) {
		got = append(got, tool.Name)
	}
	want := []string{"sablier", "nuage"}
	if !slices.Equal(got, want) {
		t.Fatalf("stale() = %v, want %v", got, want)
	}
}

func stubLatestTag(t *testing.T, fn func(string) (string, error)) {
	t.Helper()
	original := latestTagFn
	t.Cleanup(func() { latestTagFn = original })
	latestTagFn = fn
}

func stubBinDir(t *testing.T, dir string) {
	t.Helper()
	original := flagBinDir
	t.Cleanup(func() { flagBinDir = original })
	flagBinDir = dir
}

// stubBinary writes something Installed can actually execute, because the whole
// design is that facile discovers versions by running binaries, not by trusting
// a state file.
func stubBinary(t *testing.T, dir, bin, line string) {
	t.Helper()
	script := fmt.Sprintf("#!/bin/sh\necho %q\n", line)
	if err := os.WriteFile(filepath.Join(dir, bin), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
