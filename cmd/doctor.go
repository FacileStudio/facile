package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
	"github.com/FacileStudio/facile/internal/ui"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the health of the Facile installation",
	Long:  "Report anything that would make a Facile tool behave unexpectedly on this machine.",
	RunE: func(_ *cobra.Command, _ []string) error {
		problems := 0
		dir := binDir()
		ui.Step("Install directory %s", store.Tilde(dir))
		problems += checkBinDir(dir)
		problems += checkTools(dir)
		problems += checkCatalog()
		problems += checkStaged(dir)
		problems += checkSelfCopies()

		if problems == 0 {
			ui.Success("Everything looks healthy")
		}
		reportSelf()
		if problems > 0 {
			return fmt.Errorf("%d problem(s) found", problems)
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(doctorCmd) }

func checkBinDir(dir string) int {
	if _, err := os.Stat(dir); err != nil {
		ui.Warn("%s does not exist yet", store.Tilde(dir))
		return 1
	}
	if !store.OnPath(dir) {
		ui.Warn("%s is not on your PATH", store.Tilde(dir))
		ui.Hint("export PATH=\"%s:$PATH\"", dir)
		return 1
	}
	return 0
}

// checkTools runs every installed binary and looks for a same-named binary
// earlier on PATH, which is the usual cause of "I updated it and nothing changed".
func checkTools(dir string) int {
	problems := 0
	installedAny := false
	for _, tool := range catalog().Tools {
		line, ok := installer.Installed(dir, tool.Bin)
		if !ok {
			continue
		}
		installedAny = true
		ui.Success("%s", line)
		problems += checkShadow(tool, dir)
	}
	if !installedAny {
		ui.Warn("no Facile tools installed — run `facile install`")
		return problems + 1
	}
	return problems
}

// checkShadow resolves both sides before comparing. Resolving only the one
// found on PATH reports every install under a symlinked directory as its own
// impostor, and on macOS /tmp is such a directory.
func checkShadow(tool manifest.Tool, dir string) int {
	found, err := exec.LookPath(tool.Bin)
	if err != nil {
		return 0
	}
	if realPath(found) == realPath(filepath.Join(dir, tool.Bin)) {
		return 0
	}
	ui.Warn("another %s comes first on your PATH: %s", tool.Bin, found)
	return 1
}

func realPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func checkCatalog() int {
	if _, err := manifest.Refresh(store.CatalogPath()); err != nil {
		ui.Warn("cannot reach the tool catalog, working from the built-in copy")
		return 1
	}
	return 0
}

// checkSelfCopies walks PATH for facile binaries other than the running one.
// checkShadow cannot cover this: it compares against the catalog bin dir, and
// facile is not a catalog tool. The failure it catches is specific — a self
// update writes to the running binary's own directory, so a second copy earlier
// on PATH keeps answering with the old version and the update looks like it did
// nothing.
func checkSelfCopies() int {
	seen := map[string]bool{realPath(executable()): true}
	var others []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		path := filepath.Join(dir, selfTool().Bin)
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			continue
		}
		resolved := realPath(path)
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		others = append(others, path)
	}
	if len(others) == 0 {
		return 0
	}
	ui.Warn("%d other facile binary/binaries on your PATH", len(others))
	for _, path := range others {
		ui.Hint("%s", path)
	}
	return 1
}

// reportSelf resolves facile's own tag rather than reading the cache: doctor is
// the command a user runs when something is wrong, and answering that from a
// day-old file is how you tell someone they are current when they are two
// releases behind.
//
// It counts as no problem and cannot fail the command. An old facile installs
// tools perfectly well, and a health check that goes red on every release would
// be red more often than it is useful.
func reportSelf() {
	tag, outdated := selfOutdated(selfLatest(true))
	if !outdated {
		ui.Success("facile %s", version)
		return
	}
	ui.Warn("facile %s is behind %s", version, tag)
	ui.Hint("%s", upgradeHint())
}

// checkStaged looks for the temporary files an interrupted install leaves behind.
func checkStaged(dir string) int {
	matches, err := filepath.Glob(filepath.Join(dir, ".*.new.*"))
	if err != nil || len(matches) == 0 {
		return 0
	}
	ui.Warn("%d interrupted install(s) left staged files in %s", len(matches), store.Tilde(dir))
	for _, path := range matches {
		ui.Hint("rm %s", path)
	}
	return 1
}
