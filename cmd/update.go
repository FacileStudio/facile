package cmd

import (
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

// NewUpdateCommand builds the update command and its flags.
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [tool...]",
		Short: "Update installed Facile tools",
		Long: "Bring tools up to their latest release.\n\n" +
			"With no arguments it updates everything already installed. A tool that " +
			"already reports the latest published version is left alone; pass --force " +
			"to reinstall it anyway.",
		RunE: runUpdate,
	}
	cmd.Flags().Bool("all", false, "Update every tool in the catalog")
	cmd.Flags().Bool("no-skill", false, "Skip AI agent skill registration")
	cmd.Flags().Bool("force", false, "Reinstall even when already at the latest release")
	return cmd
}

func runUpdate(c *cobra.Command, args []string) error {
	wantSelf, rest, named := splitSelf(c, args)
	targets, err := updateTargets(c, rest, named)
	if err != nil {
		return err
	}
	if len(targets) == 0 && !wantSelf {
		ui.Step("No Facile tools installed")
		return nil
	}

	force, _ := c.Flags().GetBool("force")
	tools := targets
	if !force {
		tools = stale(c, targets)
	}
	var failure error
	switch {
	case len(tools) > 0:
		failure = installAll(c, tools)
	case len(targets) > 0:
		reportPath(binDir(c))
	}
	if wantSelf {
		if err := updateSelf(c); err != nil && failure == nil {
			failure = err
		}
	}
	return failure
}

var latestTagFn = installer.LatestTag

// stale drops the tools whose installed binary already reports the latest
// published version, and reports each one it dropped.
func stale(c *cobra.Command, tools []manifest.Tool) []manifest.Tool {
	dir := binDir(c)
	current := checkAll(tools, dir)

	keep := make([]manifest.Tool, 0, len(tools))
	for i, tool := range tools {
		if current[i] != "" {
			ui.Success("%s is up to date", current[i])
			continue
		}
		keep = append(keep, tool)
	}
	return keep
}

// checkAll resolves every tool's installed version in parallel.
func checkAll(tools []manifest.Tool, dir string) []string {
	current := make([]string, len(tools))
	var wg sync.WaitGroup
	for i, tool := range tools {
		wg.Go(func() {
			checkOne(tool, dir, i, current)
		})
	}
	wg.Wait()
	return current
}

// checkOne resolves one tool's installed version and its latest tag.
func checkOne(tool manifest.Tool, dir string, i int, current []string) {
	if have, ok := installer.Installed(dir, tool.Bin); ok && upToDate(have, tool) {
		current[i] = have
	}
}

// upToDate compares the version the installed binary reports against the latest
// release tag.
func upToDate(have string, tool manifest.Tool) bool {
	if have == "" || tool.Asset == "" {
		return false
	}
	tag, err := latestTagFn(tool.Repo)
	if err != nil {
		return false
	}
	expected := strings.TrimPrefix(tag, "v")
	if matchesVersion(have, tool, expected) {
		return true
	}
	installed := versionOf(have)
	if strings.Contains(installed, "dev") {
		return true
	}
	return installed != "" && installed == expected
}

// matchesVersion is true when the reported version line states the tag.
func matchesVersion(have string, tool manifest.Tool, expected string) bool {
	parts := strings.Fields(have)
	if len(parts) >= 2 && parts[0] == tool.Bin && parts[1] == expected {
		return true
	}
	return len(parts) >= 3 && parts[0] == tool.Bin && parts[1] == "version" && parts[2] == expected
}
