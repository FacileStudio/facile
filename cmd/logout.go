package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/authflow"
	"github.com/FacileStudio/facile/internal/ui"
)

var logoutCmd = &cobra.Command{
	Use:   "logout <tool...>",
	Short: "Clear stored Facile credentials",
	Long: "Remove the credential each tool stored, wherever it stored it.\n\n" +
		"The server URL is left in place, so signing in again does not ask for it twice.",
	Args: cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		tools, err := resolve(args)
		if err != nil {
			return err
		}

		var failed []string
		for _, tool := range tools {
			if !tool.NeedsLogin() {
				ui.Step("%s stores no credential", tool.Name)
				if note := tool.Note(); note != "" {
					ui.Hint("%s", note)
				}
				continue
			}

			outcome, err := authflow.Logout(tool)
			if err != nil {
				ui.Error("%s", err)
				failed = append(failed, tool.Name)
				continue
			}
			if len(outcome.Locations) == 0 {
				ui.Step("%s had no stored credential", tool.Name)
			} else {
				ui.Success("%s signed out", tool.Name)
				ui.Hint("cleared from %s", strings.Join(outcome.Locations, " and "))
			}
			authflow.WarnEnvToken(tool.Auth)
		}

		if len(failed) > 0 {
			return fmt.Errorf("%d of %d logouts failed: %s",
				len(failed), len(tools), strings.Join(failed, ", "))
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(logoutCmd) }
