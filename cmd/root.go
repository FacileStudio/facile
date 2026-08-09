package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
	"github.com/FacileStudio/facile/internal/ui"
)

var version = "dev"

var (
	flagBinDir  string
	flagNoColor bool
)

var rootCmd = &cobra.Command{
	Use:           "facile",
	Short:         "Install and manage the Facile Studio tools",
	Long:          "One command to install, update and sign in to the Facile Studio suite.",
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		if flagNoColor {
			ui.SetColor(false)
		}
	},
}

// Execute runs the CLI and maps a returned error onto the suite's exit codes.
func Execute(v string) {
	version = v
	rootCmd.Version = v
	rootCmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	if err := rootCmd.Execute(); err != nil {
		ui.Error("%s", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagBinDir, "bin-dir", "",
		"Directory to install into (default ~/.local/bin)")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable colored output")
}

func binDir() string {
	if flagBinDir != "" {
		return flagBinDir
	}
	return store.BinDir()
}

func catalog() *manifest.Manifest {
	return manifest.Load(store.CatalogPath())
}

// resolve turns command-line names into catalog entries, rejecting unknown ones
// before any work starts rather than half way through a batch.
func resolve(names []string) ([]manifest.Tool, error) {
	m := catalog()
	tools := make([]manifest.Tool, 0, len(names))
	for _, name := range names {
		tool, ok := m.Get(name)
		if !ok {
			return nil, unknownTool(name, m)
		}
		tools = append(tools, tool)
	}
	return tools, nil
}
