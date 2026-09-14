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

// NewInstallCommand builds the install command and its flags.
func NewInstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [tool...]",
		Short: "Install Facile tools",
		Long: "Install one or more tools from the Facile Studio catalog.\n\n" +
			"With no arguments it opens a picker. Pass --all to take everything.",
		RunE: func(c *cobra.Command, args []string) error {
			tools, err := chooseTools(c, args)
			if err != nil {
				return err
			}
			if len(tools) == 0 {
				ui.Step("Nothing selected")
				return nil
			}
			return installAll(c, tools)
		},
	}
	cmd.Flags().String("version", "", "Release tag to install (default latest)")
	cmd.Flags().Bool("source", false, "Build from source, ignore published releases")
	cmd.Flags().Bool("no-skill", false, "Skip AI agent skill registration")
	cmd.Flags().Bool("no-mcp", false, "Skip MCP server registration in agent harnesses")
	cmd.Flags().Bool("all", false, "Install every tool in the catalog")
	return cmd
}

func chooseTools(c *cobra.Command, args []string) ([]manifest.Tool, error) {
	if all, _ := c.Flags().GetBool("all"); all {
		m, err := catalog()
		if err != nil {
			return nil, err
		}
		return m.Tools, nil
	}
	if len(args) > 0 {
		return resolve(args)
	}
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return nil, fmt.Errorf("no tool named — pass tool names or --all when not on a terminal")
	}
	return pickTools(c)
}

// pickTools shows the catalog and lets the user check off what they want.
// Already-installed tools start checked, so the picker doubles as a review.
func pickTools(c *cobra.Command) ([]manifest.Tool, error) {
	m, err := catalog()
	if err != nil {
		return nil, err
	}
	dir := binDir(c)
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

func installAll(c *cobra.Command, tools []manifest.Tool) error {
	dir := binDir(c)
	opts := buildOptions(c, dir)

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
// buildOptions assembles the install options from the command flags and the
// catalog's MCP-tool list, whose fetch is best effort: a catalog that cannot be
// read means MCP registration is skipped, never the binary install.
func buildOptions(c *cobra.Command, dir string) installer.Options {
	version, _ := c.Flags().GetString("version")
	fromSrc, _ := c.Flags().GetBool("source")
	noSkill, _ := c.Flags().GetBool("no-skill")
	noMCP, _ := c.Flags().GetBool("no-mcp")
	opts := installer.Options{
		BinDir:    dir,
		Version:   version,
		FromSrc:   fromSrc,
		WithSkill: !noSkill,
		WithMCP:   !noMCP,
	}
	if m, err := catalog(); err == nil && m != nil {
		opts.MCPTools = m.MCP
	}
	return opts
}

func reportPath(dir string) {
	if store.OnPath(dir) {
		return
	}
	ui.Warn("%s is not on your PATH", store.Tilde(dir))
	ui.Hint("export PATH=\"%s:$PATH\"", dir)
}
