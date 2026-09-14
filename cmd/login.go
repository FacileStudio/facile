package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

// NewLoginCommand builds the login command and its flags.
func NewLoginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login [tool...]",
		Short: "Sign in to Facile tools",
		Long: "Run each tool's own login flow and write the credential where that tool " +
			"already reads it from.\n\nWith no arguments it signs in to every installed tool " +
			"that has an account. Pass --pick to choose, or --all to include tools that are " +
			"not installed yet.",
		RunE: func(c *cobra.Command, args []string) error {
			tools, err := chooseLogins(c, args)
			if err != nil {
				return err
			}
			if len(tools) == 0 {
				ui.Step("No installed tool needs a login")
				ui.Hint("run `facile install` first, or `facile login --all`")
				return nil
			}
			return loginAll(c, order(tools))
		},
	}
	cmd.Flags().String("server", "", "Server URL to sign in to")
	cmd.Flags().Bool("no-browser", false, "Print the sign-in URL instead of opening a browser")
	cmd.Flags().Bool("all", false, "Include tools that are not installed")
	cmd.Flags().Bool("pick", false, "Choose from a list instead of taking every installed tool")
	return cmd
}

// chooseLogins defaults to what is installed and needs an account. Making the
// user select from a list every time is a question with one sensible answer,
// and asking it is the friction, not the flows.
func chooseLogins(c *cobra.Command, args []string) ([]manifest.Tool, error) {
	switch {
	case len(args) > 0:
		return resolve(args)
	case loginFlag(c, "all"):
		return withAccounts()
	case loginFlag(c, "pick"):
		if !isatty.IsTerminal(os.Stdin.Fd()) {
			return nil, fmt.Errorf("--pick needs a terminal — name the tools instead")
		}
		return pickLogins()
	}
	return installedWithAccounts(c)
}

func loginFlag(c *cobra.Command, name string) bool {
	on, _ := c.Flags().GetBool(name)
	return on
}

// order runs the federated flows first. They all reach the same identity
// provider, so once one has authenticated the rest complete without the user
// touching anything — which only helps if they do not come last, after the
// prompts have already made the run feel manual. It matters more under the
// device grant than under the loopback one: there the shared thing is a code
// somebody typed, not a cookie their browser was already holding.
func order(tools []manifest.Tool) []manifest.Tool {
	var browser, rest []manifest.Tool
	for _, tool := range tools {
		if tool.Federates() {
			browser = append(browser, tool)
			continue
		}
		rest = append(rest, tool)
	}
	return append(browser, rest...)
}

// withAccounts is the subset --all and the picker offer. A tool with no login
// flow is not a candidate; naming it explicitly still prints its note.
func withAccounts() ([]manifest.Tool, error) {
	m, err := catalog()
	if err != nil {
		return nil, err
	}
	var tools []manifest.Tool
	for _, tool := range m.Tools {
		if tool.NeedsLogin() {
			tools = append(tools, tool)
		}
	}
	return tools, nil
}

// installedWithAccounts is the default set: signing in to a tool the user has
// not installed is work they did not ask for.
func installedWithAccounts(c *cobra.Command) ([]manifest.Tool, error) {
	dir := binDir(c)
	var tools []manifest.Tool
	all, err := withAccounts()
	if err != nil {
		return nil, err
	}
	for _, tool := range all {
		if _, ok := installer.Installed(dir, tool.Bin); ok {
			tools = append(tools, tool)
		}
	}
	return tools, nil
}

func pickLogins() ([]manifest.Tool, error) {
	candidates, err := withAccounts()
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no tool in the catalog has a login flow — refresh the catalog with `facile list`")
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
