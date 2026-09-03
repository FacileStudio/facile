package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
	"github.com/FacileStudio/facile/internal/ui"
)

var (
	flagJSON  bool
	flagQuiet bool
	flagCheck bool
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

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List the Facile tools and what is installed",
	Long: "Show every tool in the catalog, the version installed, and whether a " +
		"newer release is published.\n\n" +
		"The catalog is refreshed from the remote on every run, so a tool added " +
		"upstream shows up immediately. The published versions come from a cache " +
		"refreshed at most once a day, so listing stays instant and works offline. " +
		"Pass --check to resolve them now.",
	RunE: func(_ *cobra.Command, _ []string) error {
		m := catalog()
		if flagQuiet {
			printNames(survey(m, nil))
			return nil
		}
		entries := survey(m, latestTags(m))
		if flagJSON {
			return json.NewEncoder(os.Stdout).Encode(entries)
		}
		printTable(entries)
		return nil
	},
}

func init() {
	listCmd.Flags().BoolVar(&flagJSON, "json", false, "Print one JSON document to stdout")
	listCmd.Flags().BoolVarP(&flagQuiet, "quiet", "q", false, "Print installed tool names only")
	listCmd.Flags().BoolVar(&flagCheck, "check", false, "Resolve the latest releases now instead of using the cache")
	rootCmd.AddCommand(listCmd)
}

// latestTags asks only about the tools that are installed, plus facile itself.
// Resolving a release for a tool the user does not have spends a request to
// render nothing.
func latestTags(m *manifest.Manifest) map[string]string {
	dir := binDir()
	repos := []string{facileRepo}
	for _, tool := range m.Tools {
		if _, ok := installer.Installed(dir, tool.Bin); ok {
			repos = append(repos, tool.Repo)
		}
	}
	return installer.Latest(store.LatestPath(), repos, flagCheck)
}

// survey lists facile first, then the catalog in its own order. The installer
// leads because it is the one row that explains the others: a stale facile is
// the reason a tool can be missing a login flow or a whole catalog entry.
func survey(m *manifest.Manifest, latest map[string]string) []entry {
	dir := binDir()
	entries := make([]entry, 0, len(m.Tools)+1)
	entries = append(entries, selfEntry(latest))
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

// versionOf returns the version part of a line like "{tool} {version}" or
// "{tool} version {version}". It returns everything after the first space,
// unless the second word is "version", in which case it returns the third word.
func versionOf(line string) string {
	parts := strings.Fields(line)
	if len(parts) >= 3 && parts[1] == "version" {
		return parts[2]
	}
	if _, ver, found := strings.Cut(line, " "); found {
		return ver
	}
	return line
}
