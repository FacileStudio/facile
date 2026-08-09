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

		if problems > 0 {
			return fmt.Errorf("%d problem(s) found", problems)
		}
		ui.Success("Everything looks healthy")
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

func checkShadow(tool manifest.Tool, dir string) int {
	found, err := exec.LookPath(tool.Bin)
	if err != nil {
		return 0
	}
	resolved, _ := filepath.EvalSymlinks(found)
	if resolved == "" {
		resolved = found
	}
	if resolved == filepath.Join(dir, tool.Bin) {
		return 0
	}
	ui.Warn("another %s comes first on your PATH: %s", tool.Bin, found)
	return 1
}

func checkCatalog() int {
	if _, err := manifest.Refresh(store.CatalogPath()); err != nil {
		ui.Warn("cannot reach the tool catalog, working from the built-in copy")
		return 1
	}
	return 0
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
