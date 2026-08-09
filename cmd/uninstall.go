package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/store"
	"github.com/FacileStudio/facile/internal/ui"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <tool...>",
	Short: "Remove installed Facile tools",
	Long: "Remove the binaries of one or more tools.\n\n" +
		"Configuration and stored credentials are left alone; use `facile logout` for those.",
	Args: cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		tools, err := resolve(args)
		if err != nil {
			return err
		}
		dir := binDir()
		removed := 0
		for _, tool := range tools {
			gone, err := installer.Uninstall(dir, tool.Bin)
			if err != nil {
				return err
			}
			if !gone {
				ui.Warn("%s is not installed in %s", tool.Name, store.Tilde(dir))
				continue
			}
			ui.Success("%s removed", tool.Name)
			removed++
		}
		if removed == 0 {
			return fmt.Errorf("nothing to remove")
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(uninstallCmd) }
