package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
	"github.com/FacileStudio/facile/internal/ui"
)

var updateCmd = &cobra.Command{
	Use:   "update [tool...]",
	Short: "Update installed Facile tools",
	Long: "Reinstall tools at their latest release.\n\n" +
		"With no arguments it updates everything already installed.",
	RunE: func(_ *cobra.Command, args []string) error {
		tools, err := updateTargets(args)
		if err != nil {
			return err
		}
		if len(tools) == 0 {
			ui.Step("No Facile tools installed")
			return nil
		}
		if _, err := manifest.Refresh(store.CatalogPath()); err != nil {
			ui.Warn("could not refresh the tool catalog, using the local copy")
		}
		return installAll(tools)
	},
}

func init() {
	updateCmd.Flags().BoolVar(&flagAll, "all", false, "Update every tool in the catalog")
	updateCmd.Flags().BoolVar(&flagNoSkill, "no-skill", false, "Skip AI agent skill registration")
	rootCmd.AddCommand(updateCmd)
}

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
