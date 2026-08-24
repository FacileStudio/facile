package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
)

// facileRepo is facile's own repository, kept here rather than in the catalog.
// The catalog describes tools that live in the bin dir and are discovered by
// running them, and facile may be neither: a Homebrew cask puts it under the
// brew prefix, where a catalog lookup would report it as not installed while
// it is the binary doing the reporting.
const facileRepo = "FacileStudio/facile"

// selfTool describes facile to the installer. The asset name matches the
// goreleaser archive, so the release path resolves
// facile_<version>_<os>_<arch>.tar.gz exactly as it does for a catalog tool.
// No skill key, so nothing is registered with the agents on this machine.
func selfTool() manifest.Tool {
	return manifest.Tool{
		Name:      "facile",
		Summary:   "The suite installer",
		Repo:      facileRepo,
		Branch:    "main",
		Bin:       "facile",
		Build:     "go",
		SrcSubdir: ".",
		Asset:     "facile",
	}
}

// selfEntry renders facile as a listing row. The version is the one compiled
// into the running binary rather than one read off disk, which is the strongest
// form of "verify by running" available: it is not a report about some binary,
// it is the binary reporting.
func selfEntry(latest map[string]string) entry {
	tool := selfTool()
	e := entry{
		Name:      tool.Name,
		Summary:   tool.Summary,
		Repo:      tool.Repo,
		Installed: true,
		Version:   version,
	}
	e.Latest = strings.TrimPrefix(latest[facileRepo], "v")
	e.Outdated = outdated(e.Version, e.Latest)
	return e
}

func selfOutdated(latest map[string]string) (string, bool) {
	tag := strings.TrimPrefix(latest[facileRepo], "v")
	return tag, outdated(version, tag)
}

// selfDir is the directory holding the running binary, and the only place a
// self-update may write. Symlinks are resolved first so the target is the real
// file rather than a link pointing at it, which is what CLI-STANDARD §3.1 means
// by replacing its own binary at its own path.
//
// It refuses under Homebrew. brew records the version it staged, so overwriting
// the file in place leaves brew's manifest claiming the old one — and the next
// `brew upgrade` re-stages from that record and quietly reverts the update.
func selfDir() (string, bool) {
	path := realPath(executable())
	if path == "" || fromHomebrew(path) {
		return "", false
	}
	return filepath.Dir(path), true
}

// upgradeHint names the command that actually replaces this binary.
func upgradeHint() string {
	if _, ok := selfDir(); !ok {
		return "brew upgrade --cask facile"
	}
	return "facile update facile"
}

// fromHomebrew matches on the path a cask or formula actually resolves to, not
// on the symlink in the brew bin dir. `/usr/local/bin/facile` is named by
// nothing and points into `/usr/local/Cellar`, so the caller resolves symlinks
// first and this only has to recognise the destination. Matching a bare
// "homebrew" anywhere in the path would claim a checked-out tap as an install.
func fromHomebrew(path string) bool {
	for _, dir := range []string{"/Caskroom/", "/Cellar/"} {
		if strings.Contains(path, dir) {
			return true
		}
	}
	for _, prefix := range []string{"/opt/homebrew/", "/usr/local/Homebrew/", "/home/linuxbrew/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func executable() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Clean(path)
}

var semver = regexp.MustCompile(`^\d+\.\d+\.\d+`)

// outdated compares what a binary reports against the latest published tag, and
// answers no whenever the question cannot be asked. A source build reports a
// commit SHA, and a SHA cannot be ordered against a tag — it may be newer.
//
// `update` draws the opposite conclusion from the same unknown on purpose:
// there, an unresolved comparison reinstalls, because reinstalling is safe and
// self-correcting. Here the output is a claim on screen, and an unverifiable
// claim is worse than no claim. A marker that says a locally built binary is
// behind also sends the reader to `facile update`, which would replace their
// build with the release.
func outdated(have, latest string) bool {
	return latest != "" && semver.MatchString(have) && have != latest
}

// selfLatest resolves facile's own tag through the same cache the tool listing
// uses, so `doctor` costs nothing when `list` has already refreshed it.
func selfLatest(force bool) map[string]string {
	return installer.Latest(store.LatestPath(), []string{facileRepo}, force)
}
