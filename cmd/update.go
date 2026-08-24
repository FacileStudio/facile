package cmd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
	"github.com/FacileStudio/facile/internal/ui"
)

var (
	flagCatalogOnly bool
	flagForce       bool
)

var updateCmd = &cobra.Command{
	Use:   "update [tool...]",
	Short: "Update installed Facile tools",
	Long: "Bring tools up to their latest release.\n\n" +
		"With no arguments it updates everything already installed. A tool that " +
		"already reports the latest published version is left alone; pass --force " +
		"to reinstall it anyway.",
	RunE: func(_ *cobra.Command, args []string) error {
		if flagCatalogOnly {
			m, err := manifest.Refresh(store.CatalogPath())
			if err != nil {
				return fmt.Errorf("cannot reach the tool catalog — %s", err)
			}
			ui.Success("Catalog refreshed, %d tools", len(m.Tools))
			return nil
		}

		if _, err := manifest.Refresh(store.CatalogPath()); err != nil {
			ui.Warn("could not refresh the tool catalog, using the local copy")
		}
		targets, err := updateTargets(args)
		if err != nil {
			return err
		}
		if len(targets) == 0 {
			ui.Step("No Facile tools installed")
			return nil
		}
		tools := targets
		if !flagForce {
			tools = stale(targets)
		}
		if len(tools) == 0 {
			reportPath(binDir())
			return nil
		}
		return installAll(tools)
	},
}

func init() {
	updateCmd.Flags().BoolVar(&flagAll, "all", false, "Update every tool in the catalog")
	updateCmd.Flags().BoolVar(&flagCatalogOnly, "catalog", false, "Refresh the tool catalog and change nothing else")
	updateCmd.Flags().BoolVar(&flagNoSkill, "no-skill", false, "Skip AI agent skill registration")
	updateCmd.Flags().BoolVar(&flagForce, "force", false, "Reinstall even when already at the latest release")
	rootCmd.AddCommand(updateCmd)
}

// latestTagFn is a seam for the tests, which must decide what to skip without
// reaching GitHub.
var latestTagFn = installer.LatestTag

// stale drops the tools whose installed binary already reports the latest
// published version, and reports each one it dropped. Resolving a tag is one
// redirect; reinstalling costs an archive, a checksums file, and a rewrite of
// every agent skill on the machine, for a byte-identical binary.
//
// The checks run concurrently, because ten sequential redirects cost about
// eight seconds on a run that installs nothing. Results land in a slice indexed
// by position rather than a channel, so the report stays in catalog order.
func stale(tools []manifest.Tool) []manifest.Tool {
	dir := binDir()
	current := make([]string, len(tools))
	var wg sync.WaitGroup
	for i, tool := range tools {
		wg.Go(func() {
			if have, ok := installer.Installed(dir, tool.Bin); ok && upToDate(have, tool) {
				current[i] = have
			}
		})
	}
	wg.Wait()

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

// upToDate compares the version the installed binary reports against the latest
// release tag. Anything it cannot establish — not installed, no release asset,
// an unreadable tag, a version line that is not a plain semver — is not up to
// date, so the run falls through to the reinstall facile did unconditionally
// before. Skipping is the claim that needs evidence.
func upToDate(have string, tool manifest.Tool) bool {
	if have == "" || tool.Asset == "" {
		return false
	}
	tag, err := latestTagFn(tool.Repo)
	if err != nil {
		return false
	}
	installed := versionOf(have)
	return installed != "" && installed == strings.TrimPrefix(tag, "v")
}

// updateTargets reads the catalog after Refresh has written it, so a tool added
// upstream since the last run is selectable now rather than one run later.
func updateTargets(args []string) ([]manifest.Tool, error) {
	if len(args) > 0 {
		return resolve(args)
	}
	if flagAll {
		return catalog().Tools, nil
	}
	return installedTools(), nil
}

func installedTools() []manifest.Tool {
	dir := binDir()
	var tools []manifest.Tool
	for _, tool := range catalog().Tools {
		if _, ok := installer.Installed(dir, tool.Bin); ok {
			tools = append(tools, tool)
		}
	}
	return tools
}

func unknownTool(name string, m *manifest.Manifest) error {
	return fmt.Errorf("unknown tool: %s — run `facile list` to see the catalog", name)
}
