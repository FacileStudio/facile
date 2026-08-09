package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
	"github.com/FacileStudio/facile/internal/ui"
)

var (
	flagVersion string
	flagSource  bool
	flagNoSkill bool
	flagAll     bool
)

var installCmd = &cobra.Command{
	Use:   "install [tool...]",
	Short: "Install Facile tools",
	Long: "Install one or more tools from the Facile Studio catalog.\n\n" +
		"With no arguments it opens a picker. Pass --all to take everything.",
	RunE: func(_ *cobra.Command, args []string) error {
		tools, err := chooseTools(args)
		if err != nil {
			return err
		}
		if len(tools) == 0 {
			ui.Step("Nothing selected")
			return nil
		}
		return installAll(tools)
	},
}

func init() {
	installCmd.Flags().StringVar(&flagVersion, "version", "", "Release tag to install (default latest)")
	installCmd.Flags().BoolVar(&flagSource, "source", false, "Build from source, ignore published releases")
	installCmd.Flags().BoolVar(&flagNoSkill, "no-skill", false, "Skip AI agent skill registration")
	installCmd.Flags().BoolVar(&flagAll, "all", false, "Install every tool in the catalog")
	rootCmd.AddCommand(installCmd)
}

func chooseTools(args []string) ([]manifest.Tool, error) {
	if flagAll {
		return catalog().Tools, nil
	}
	if len(args) > 0 {
		return resolve(args)
	}
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return nil, fmt.Errorf("no tool named — pass tool names or --all when not on a terminal")
	}
	return pickTools()
}

// pickTools shows the catalog and lets the user check off what they want.
// Already-installed tools start checked, so the picker doubles as a review.
func pickTools() ([]manifest.Tool, error) {
	m := catalog()
	dir := binDir()
	options := make([]huh.Option[string], 0, len(m.Tools))
	for _, tool := range m.Tools {
		_, present := installer.Installed(dir, tool.Bin)
		label := fmt.Sprintf("%-9s %s", tool.Name, ui.Dim(tool.Summary))
		options = append(options, huh.NewOption(label, tool.Name).Selected(present))
	}

	var chosen []string
	form := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Facile Studio").
			Description("Space to toggle, enter to install").
			Options(options...).
			Value(&chosen),
	))
	if err := form.Run(); err != nil {
		return nil, err
	}
	return resolve(chosen)
}

func installAll(tools []manifest.Tool) error {
	dir := binDir()
	opts := installer.Options{
		BinDir:    dir,
		Version:   flagVersion,
		FromSrc:   flagSource,
		WithSkill: !flagNoSkill,
	}

	var failed []string
	for _, tool := range tools {
		ui.Step("Installing %s", tool.Name)
		reported, err := installer.Install(tool, opts)
		if err != nil {
			ui.Error("%s", err)
			failed = append(failed, tool.Name)
			continue
		}
		ui.Success("%s installed to %s/%s", reported, store.Tilde(dir), tool.Bin)
	}

	reportPath(dir)
	if len(failed) > 0 {
		return fmt.Errorf("%d of %d tools failed: %s",
			len(failed), len(tools), strings.Join(failed, ", "))
	}
	return nil
}

// reportPath warns but never edits the user's shell configuration.
func reportPath(dir string) {
	if store.OnPath(dir) {
		return
	}
	ui.Warn("%s is not on your PATH", store.Tilde(dir))
	ui.Hint("export PATH=\"%s:$PATH\"", dir)
}
