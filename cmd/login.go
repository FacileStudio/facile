package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/authflow"
	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

var (
	flagServer    string
	flagNoBrowser bool
	flagLoginAll  bool
	flagPick      bool
)

var loginCmd = &cobra.Command{
	Use:   "login [tool...]",
	Short: "Sign in to Facile tools",
	Long: "Run each tool's own login flow and write the credential where that tool " +
		"already reads it from.\n\nWith no arguments it signs in to every installed tool " +
		"that has an account. Pass --pick to choose, or --all to include tools that are " +
		"not installed yet.",
	RunE: func(_ *cobra.Command, args []string) error {
		tools, err := chooseLogins(args)
		if err != nil {
			return err
		}
		if len(tools) == 0 {
			ui.Step("No installed tool needs a login")
			ui.Hint("run `facile install` first, or `facile login --all`")
			return nil
		}
		return loginAll(order(tools))
	},
}

func init() {
	loginCmd.Flags().StringVar(&flagServer, "server", "", "Server URL to sign in to")
	loginCmd.Flags().BoolVar(&flagNoBrowser, "no-browser", false, "Print the sign-in URL instead of opening a browser")
	loginCmd.Flags().BoolVar(&flagLoginAll, "all", false, "Include tools that are not installed")
	loginCmd.Flags().BoolVar(&flagPick, "pick", false, "Choose from a list instead of taking every installed tool")
	rootCmd.AddCommand(loginCmd)
}

// chooseLogins defaults to what is installed and needs an account. Making the
// user select from a list every time is a question with one sensible answer,
// and asking it is the friction, not the flows.
func chooseLogins(args []string) ([]manifest.Tool, error) {
	switch {
	case len(args) > 0:
		return resolve(args)
	case flagLoginAll:
		return withAccounts(), nil
	case flagPick:
		if !isatty.IsTerminal(os.Stdin.Fd()) {
			return nil, fmt.Errorf("--pick needs a terminal — name the tools instead")
		}
		return pickLogins()
	}
	return installedWithAccounts(), nil
}

// order runs the browser flows first. They all federate to the same identity
// provider, so once one has authenticated the rest complete without the user
// touching anything — which only helps if they do not come last, after the
// prompts have already made the run feel manual.
func order(tools []manifest.Tool) []manifest.Tool {
	var browser, rest []manifest.Tool
	for _, tool := range tools {
		if tool.AuthKind() == "sso" {
			browser = append(browser, tool)
			continue
		}
		rest = append(rest, tool)
	}
	return append(browser, rest...)
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

// installedWithAccounts is the default set: signing in to a tool the user has
// not installed is work they did not ask for.
func installedWithAccounts() []manifest.Tool {
	dir := binDir()
	var tools []manifest.Tool
	for _, tool := range withAccounts() {
		if _, ok := installer.Installed(dir, tool.Bin); ok {
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
