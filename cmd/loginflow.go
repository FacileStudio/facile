package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/authflow"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

// loginAll signs the user in to every tool in the batch, remembering which ones
// failed so the command can report a count rather than stopping at the first.
func loginAll(c *cobra.Command, tools []manifest.Tool) error {
	server, _ := c.Flags().GetString("server")
	noBrowser, _ := c.Flags().GetBool("no-browser")
	opts := authflow.Options{Server: server, NoBrowser: noBrowser, Session: authflow.NewSession()}

	var failed []string
	for _, tool := range tools {
		if !tool.NeedsLogin() {
			ui.Step("%s needs no login", tool.Name)
			if note := tool.Note(); note != "" {
				ui.Hint("%s", note)
			}
			continue
		}

		ui.Step("Signing in to %s", tool.Name)
		outcome, err := authflow.Login(tool, opts)
		if err != nil {
			ui.Error("%s", err)
			failed = append(failed, tool.Name)
			continue
		}
		report(tool, outcome)
	}

	if len(failed) > 0 {
		return fmt.Errorf("%d of %d logins failed: %s",
			len(failed), len(tools), strings.Join(failed, ", "))
	}
	return nil
}

// report prints one login outcome and the degraded states that rode along with
// it, so a successful run still surfaces where the token could not go.
func report(tool manifest.Tool, outcome authflow.Outcome) {
	switch {
	case outcome.Passwordless:
		ui.Success("%s connected to %s", tool.Name, outcome.ServerURL)
		ui.Hint("this instance needs no password, so every caller is served as the admin")
	case outcome.Identity != "":
		ui.Success("%s signed in as %s at %s", tool.Name, outcome.Identity, outcome.ServerURL)
	default:
		ui.Success("%s signed in at %s", tool.Name, outcome.ServerURL)
	}

	if len(outcome.Locations) > 0 {
		ui.Hint("stored in %s", strings.Join(outcome.Locations, " and "))
	}
	if outcome.KeychainFallback != "" {
		ui.Warn("your keychain is unavailable, so the token went to %s instead", outcome.KeychainFallback)
		if env := tool.EnvToken(); env != "" {
			ui.Hint("%s will not read that file — export %s=$(cat %s)", tool.Bin, env, outcome.KeychainFallback)
		}
	}
}
