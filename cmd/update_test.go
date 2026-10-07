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
		{"a version line with 'version' keyword now parses correctly", "opus version 0.1.0", "v0.1.0", nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubLatestTag(t, func(string) (string, error) { return tc.tag, tc.err })
			if got := upToDate(tc.have, "", tool); got != tc.want {
				t.Fatalf("upToDate(%q, tag %q) = %v, want %v", tc.have, tc.tag, got, tc.want)
			}
		})
	}
}

// A tool with no release asset can still be compared against the latest tag.
// The binary reports its own version; the tag comes from GitHub releases. The
// two are independent of whether a binary was published as an archive.
func TestUpToDateChecksSourceOnlyToolsAgainstTheLatestTag(t *testing.T) {
	stubLatestTag(t, func(string) (string, error) { return "v0.1.0", nil })

	tool := manifest.Tool{Name: "opus", Bin: "opus", Repo: "FacileStudio/opus"}

	t.Run("matching version is up to date", func(t *testing.T) {
		if !upToDate("opus 0.1.0", "", tool) {
			t.Fatal("a source-only tool whose version matches the latest tag should be up to date")
		}
	})

	t.Run("mismatched version is not up to date", func(t *testing.T) {
		if upToDate("opus 0.0.9", "", tool) {
			t.Fatal("a source-only tool whose version is behind should not be up to date")
		}
	})
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
			"FacileStudio/filet":   "v0.6.1",
		}[repo], nil
	})
	stubBinary(t, dir, "opus", "opus 0.1.0")
	stubBinary(t, dir, "sablier", "sablier 0.1.1")
	stubBinary(t, dir, "filet", "filet 0.6.1")

	tools := []manifest.Tool{
		{Name: "opus", Bin: "opus", Asset: "opus", Repo: "FacileStudio/opus"},
		{Name: "sablier", Bin: "sablier", Asset: "sablier", Repo: "FacileStudio/Sablier"},
		{Name: "nuage", Bin: "nuage", Asset: "nuage", Repo: "FacileStudio/Nuage"},
		{Name: "filet", Bin: "filet", Asset: "filet", Repo: "FacileStudio/filet"},
	}

	var got []string
	for _, tool := range stale(NewUpdateCommand(), tools) {
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
	t.Setenv("FACILE_BIN_DIR", dir)
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

func TestUpToDateWithVersionPattern(t *testing.T) {
	dir := t.TempDir()

	stubBinary(t, dir, "opus", "opus v1.2.3 (abc123)")
	stubBinary(t, dir, "opus-old", "opus v1.1.0 (def456)")
	stubBinary(t, dir, "baz", "baz tool 2.0.0 linux/amd64")

	cases := []struct {
		path, have, versionPattern, tag string
		want                            bool
	}{
		{filepath.Join(dir, "opus"), "opus v1.2.3 (abc123)", `v(\d+\.\d+\.\d+)`, "v1.2.3", true},
		{filepath.Join(dir, "opus-old"), "opus v1.1.0 (def456)", `v(\d+\.\d+\.\d+)`, "v1.2.3", false},
		{filepath.Join(dir, "baz"), "baz tool 2.0.0 linux/amd64", `\d+\.\d+\.\d+`, "v2.0.0", true},
		{filepath.Join(dir, "opus"), "opus v1.2.3 (abc123)", `release-(\d+\.\d+\.\d+)`, "v1.2.3", false},
	}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			tool := manifest.Tool{
				Name: "opus", Bin: "opus", Asset: "opus", Repo: "FacileStudio/opus",
				VersionDetection: manifest.VersionDetection{Pattern: tc.versionPattern},
			}
			stubLatestTag(t, func(string) (string, error) { return tc.tag, nil })
			if got := upToDate(tc.have, tc.path, tool); got != tc.want {
				t.Fatalf("upToDate(%q, %q, pattern %q, tag %q) = %v, want %v",
					tc.have, tc.path, tc.versionPattern, tc.tag, got, tc.want)
			}
		})
	}
}

func TestUpToDateWithVersionCmd(t *testing.T) {
	dir := t.TempDir()
	missingDir := t.TempDir()

	stubBinary(t, dir, "opus", "opus v1.2.3 (abc123)")

	t.Run("versionCmd runs a non-default flag", func(t *testing.T) {
		tool := manifest.Tool{
			Name: "opus", Bin: "opus", Asset: "opus", Repo: "FacileStudio/opus",
			VersionDetection: manifest.VersionDetection{Cmd: "version", Pattern: `v(\d+\.\d+\.\d+)`},
		}
		stubLatestTag(t, func(string) (string, error) { return "v1.2.3", nil })
		if got := upToDate("opus 1.2.3", filepath.Join(dir, "opus"), tool); !got {
			t.Fatal("versionCmd 'version' should return v1.2.3 and match tag v1.2.3")
		}
	})

	t.Run("versionCmd fails, falls through to heuristic", func(t *testing.T) {
		tool := manifest.Tool{
			Name: "opus", Bin: "opus", Asset: "opus", Repo: "FacileStudio/opus",
			VersionDetection: manifest.VersionDetection{Cmd: "version", Pattern: `v(\d+\.\d+\.\d+)`},
		}
		stubLatestTag(t, func(string) (string, error) { return "v1.2.3", nil })
		if got := upToDate("opus 1.2.3", filepath.Join(missingDir, "missing"), tool); !got {
			t.Fatal("fallback heuristic should match 'opus 1.2.3' against tag v1.2.3")
		}
	})
}
