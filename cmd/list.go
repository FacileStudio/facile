package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

type entry struct {
	Name      string `json:"name"`
	Summary   string `json:"summary"`
	Repo      string `json:"repo"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	Latest    string `json:"latest,omitempty"`
	Outdated  bool   `json:"outdated"`
}

// NewListCommand builds the list command and its flags.
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the Facile tools and what is installed",
		Long: "Show every tool in the catalog, the version installed, and whether a " +
			"newer release is published.\n\n" +
			"The catalog is refreshed from the remote on every run, so a tool added " +
			"upstream shows up immediately.",
		RunE: runList,
	}
	cmd.Flags().Bool("json", false, "Print one JSON document to stdout")
	cmd.Flags().BoolP("quiet", "q", false, "Print installed tool names only")
	return cmd
}

func runList(c *cobra.Command, _ []string) error {
	m, err := catalog()
	if err != nil {
		return err
	}
	version := rootVersion(c)
	if quiet, _ := c.Flags().GetBool("quiet"); quiet {
		printNames(survey(c, m, nil, version))
		return nil
	}
	entries := survey(c, m, latestTags(c, m), version)
	if asJSON, _ := c.Flags().GetBool("json"); asJSON {
		return json.NewEncoder(os.Stdout).Encode(entries)
	}
	printTable(entries)
	return nil
}

// latestTags asks only about the tools that are installed, plus facile itself.
// Resolving a release for a tool the user does not have spends a request to
// render nothing.
func latestTags(c *cobra.Command, m *manifest.Manifest) map[string]string {
	dir := binDir(c)
	repos := []string{facileRepo}
	for _, tool := range m.Tools {
		if _, ok := installer.Installed(dir, tool.Bin); ok {
			repos = append(repos, tool.Repo)
		}
	}
	return installer.Latest(repos)
}

// survey lists facile first, then the catalog in its own order. The installer
// leads because it is the one row that explains the others: a stale facile is
// the reason a tool can be missing a login flow or a whole catalog entry.
func survey(c *cobra.Command, m *manifest.Manifest, latest map[string]string, version string) []entry {
	dir := binDir(c)
	entries := make([]entry, 0, len(m.Tools)+1)
	entries = append(entries, selfEntry(latest, version))
	for _, tool := range m.Tools {
		e := entry{Name: tool.Name, Summary: tool.Summary, Repo: tool.Repo}
		if line, ok := installer.Installed(dir, tool.Bin); ok {
			e.Installed = true
			e.Version = versionOf(line)
			e.Latest = strings.TrimPrefix(latest[tool.Repo], "v")
			e.Outdated = outdated(e.Version, e.Latest)
		}
		entries = append(entries, e)
	}
	return entries
}

func printNames(entries []entry) {
	for _, e := range entries {
		if e.Installed {
			ui.Out("%s", e.Name)
		}
	}
}

// printTable pads on the plain text before colorizing, since ANSI escapes have
// width in a format verb but none on screen.
func printTable(entries []entry) {
	nameWidth, stateWidth, count := 0, 0, 0
	for _, e := range entries {
		nameWidth = max(nameWidth, len(e.Name))
		stateWidth = max(stateWidth, len(stateOf(e)))
		if e.Outdated {
			count++
		}
	}
	for _, e := range entries {
		state := fmt.Sprintf("%-*s", stateWidth, stateOf(e))
		switch {
		case e.Outdated:
			state = ui.Accent(state)
		case !e.Installed:
			state = ui.Dim(state)
		case strings.Contains(e.Version, "dev"):
			state = ui.Notice(state)
		case !isSemver(e.Version):
			state = ui.Alert(state)
		default:
			state = ui.Good(state)
		}
		fmt.Printf("%-*s  %s  %s\n", nameWidth, e.Name, state, ui.Dim(e.Summary))
	}
	if count == 1 {
		ui.Out("%s", ui.Dim("1 update available, run `facile update`"))
	} else if count > 1 {
		ui.Out("%s", ui.Dim(fmt.Sprintf("%d updates available, run `facile update`", count)))
	}
	printBrewNote(entries)
}

// printBrewNote covers the one case the footer above cannot: a Homebrew facile
// is outdated but `facile update` will not touch it, so counting it under an
// instruction that cannot fix it would be a lie by arithmetic.
func printBrewNote(entries []entry) {
	if _, ok := selfDir(); ok {
		return
	}
	for _, e := range entries {
		if e.Name == "facile" && e.Outdated {
			ui.Out("%s", ui.Dim("facile is a Homebrew cask, run `"+upgradeHint()+"`"))
			return
		}
	}
}

func stateOf(e entry) string {
	switch {
	case e.Outdated:
		return e.Version + " → " + e.Latest
	case e.Installed:
		return e.Version
	default:
		return "not installed"
	}
}
