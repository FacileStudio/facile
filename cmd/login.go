package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/authflow"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

var (
	flagServer    string
	flagNoBrowser bool
	flagLoginAll  bool
)

var loginCmd = &cobra.Command{
	Use:   "login [tool...]",
	Short: "Sign in to Facile tools",
	Long: "Run each tool's own login flow and write the credential where that tool " +
		"already reads it from.\n\nWith no arguments it opens a picker of the tools " +
		"that have accounts. Pass --all to sign in to every one of them.",
	RunE: func(_ *cobra.Command, args []string) error {
		tools, err := chooseLogins(args)
		if err != nil {
			return err
		}
		if len(tools) == 0 {
			ui.Step("Nothing selected")
			return nil
		}
		return loginAll(tools)
	},
}

func init() {
	loginCmd.Flags().StringVar(&flagServer, "server", "", "Server URL to sign in to")
	loginCmd.Flags().BoolVar(&flagNoBrowser, "no-browser", false, "Print the sign-in URL instead of opening a browser")
	loginCmd.Flags().BoolVar(&flagLoginAll, "all", false, "Sign in to every tool that has an account")
	rootCmd.AddCommand(loginCmd)
}

func chooseLogins(args []string) ([]manifest.Tool, error) {
	if flagLoginAll {
		return withAccounts(), nil
	}
	if len(args) > 0 {
		return resolve(args)
	}
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return nil, fmt.Errorf("no tool named — pass tool names or --all when not on a terminal")
	}
	return pickLogins()
}

// withAccounts is the subset --all and the picker offer. A tool with no login
// flow is not a candidate; naming it explicitly still prints its note.
func withAccounts() []manifest.Tool {
	var tools []manifest.Tool
	for _, tool := range catalog().Tools {
		if tool.NeedsLogin() {
			tools = append(tools, tool)
		}
	}
	return tools
}

func pickLogins() ([]manifest.Tool, error) {
	candidates := withAccounts()
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no tool in the catalog has a login flow — refresh the catalog with `facile update --catalog`")
	}

	options := make([]huh.Option[string], 0, len(candidates))
	for _, tool := range candidates {
		label := fmt.Sprintf("%-9s %s", tool.Name, ui.Dim(tool.Summary))
		options = append(options, huh.NewOption(label, tool.Name))
	}

	var chosen []string
	form := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Facile Studio").
			Description("Space to toggle, enter to sign in").
			Options(options...).
			Value(&chosen),
	))
	if err := form.Run(); err != nil {
		return nil, err
	}
	return resolve(chosen)
}

func loginAll(tools []manifest.Tool) error {
	opts := authflow.Options{Server: flagServer, NoBrowser: flagNoBrowser}

	var failed []string
	for _, tool := range tools {
		// A tool that cannot be logged into is information, not a failure:
		// "capsule needs no login" is the answer to the question asked.
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
