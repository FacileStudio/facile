package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
	"github.com/FacileStudio/facile/internal/ui"
)

// buildCommand assembles the root command and its persistent flags. Every
// subcommand is constructed explicitly here rather than registered from a
// magic init(), so the command tree is visible in one place.
func buildCommand(v string) *cobra.Command {
	root := &cobra.Command{
		Use:           "facile",
		Short:         "Install and manage the Facile Studio tools",
		Long:          "One command to install, update and sign in to the Facile Studio suite.",
		Version:       v,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			if on, _ := c.Flags().GetBool("no-color"); on {
				ui.SetColor(false)
			}
		},
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.PersistentFlags().Bool("no-color", false, "Disable colored output")
	root.PersistentFlags().String("bin-dir", "", "Directory to install into (default ~/.local/bin)")
	root.AddCommand(NewInstallCommand())
	root.AddCommand(NewListCommand())
	root.AddCommand(NewLoginCommand())
	root.AddCommand(NewLogoutCommand())
	root.AddCommand(NewUninstallCommand())
	root.AddCommand(NewDoctorCommand())
	root.AddCommand(NewUpdateCommand())
	return root
}

// Execute runs the CLI and maps a returned error onto the suite's exit codes.
func Execute(v string) {
	root := buildCommand(v)
	if err := root.Execute(); err != nil {
		ui.Error("%s", err)
		os.Exit(1)
	}
}

// binDir resolves the install directory, honoring --bin-dir and falling back to
// the default.
func binDir(c *cobra.Command) string {
	dir, _ := c.Flags().GetString("bin-dir")
	if dir != "" {
		return dir
	}
	return store.BinDir()
}

// rootVersion resolves the version string stamped on the root command.
func rootVersion(c *cobra.Command) string {
	if c == nil {
		return ""
	}
	return c.Root().Version
}

// catalog returns the freshest catalog it can, and the last-resort embedded
// copy when the network is down. Only a binary whose embedded catalog is broken
// — a bad build — leaves nothing to answer with.
func catalog() (*manifest.Manifest, error) {
	if os.Getenv("FACILE_CATALOG") != "" {
		return manifest.Load(store.CatalogPath())
	}
	if m, err := manifest.Refresh(store.CatalogPath()); err == nil {
		return m, nil
	}
	return manifest.Load(store.CatalogPath())
}

// resolve turns command-line names into catalog entries, rejecting unknown ones
// before any work starts rather than half way through a batch.
func resolve(names []string) ([]manifest.Tool, error) {
	m, err := catalog()
	if err != nil {
		return nil, err
	}
	tools := make([]manifest.Tool, 0, len(names))
	for _, name := range names {
		tool, ok := m.Get(name)
		if !ok {
			return nil, unknownTool(name)
		}
		tools = append(tools, tool)
	}
	return tools, nil
}
