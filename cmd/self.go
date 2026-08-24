package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/store"
)

// facileRepo is facile's own repository, kept here rather than in the catalog.
// Every command that takes a tool name reads the catalog, so a row for facile
// would offer to uninstall the running binary. It would also let an update
// write a second copy into the bin dir while a Homebrew one still sits earlier
// on PATH.
const facileRepo = "FacileStudio/facile"

// selfOutdated reports the newest published facile when the running one is
// behind it. A source build reports "dev" rather than a semver and is never
// called outdated, because there is no tag to compare a working tree against.
func selfOutdated(latest map[string]string) (string, bool) {
	if version == "dev" || version == "" {
		return "", false
	}
	tag := strings.TrimPrefix(latest[facileRepo], "v")
	return tag, tag != "" && tag != version
}

// upgradeHint names the command that actually replaces this binary. facile
// ships as a Homebrew cask as well as through install.sh, and a cask lives
// under the brew prefix while `facile update` writes to ~/.local/bin — telling
// a brew user to re-run the installer produces a second copy and a PATH race,
// which is the "I updated it and nothing changed" bug facile warns everyone
// else about.
func upgradeHint() string {
	if fromHomebrew(realPath(executable())) {
		return "brew upgrade --cask facile"
	}
	return "curl -fsSL https://get.facile.studio | bash"
}

// fromHomebrew matches on the path a cask or formula actually resolves to, not
// on the symlink in the brew bin dir. `/usr/local/bin/facile` is indexed by
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

// selfLatest resolves facile's own latest tag through the same cache the tool
// listing uses, so `doctor` costs nothing when `list` has already refreshed it.
func selfLatest(force bool) map[string]string {
	return installer.Latest(store.LatestPath(), []string{facileRepo}, force)
}
