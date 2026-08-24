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
		"The published versions come from a cache refreshed at most once a day, so " +
		"listing stays instant and works offline. Pass --check to resolve them now.",
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

// latestTags asks only about the tools that are installed. Resolving a release
// for a tool the user does not have spends a request to render nothing.
func latestTags(m *manifest.Manifest) map[string]string {
	dir := binDir()
	var repos []string
	for _, tool := range m.Tools {
		if _, ok := installer.Installed(dir, tool.Bin); ok {
			repos = append(repos, tool.Repo)
		}
	}
	if len(repos) == 0 {
		return nil
	}
	return installer.Latest(store.LatestPath(), repos, flagCheck)
}

func survey(m *manifest.Manifest, latest map[string]string) []entry {
	dir := binDir()
	entries := make([]entry, 0, len(m.Tools))
	for _, tool := range m.Tools {
		e := entry{Name: tool.Name, Summary: tool.Summary, Repo: tool.Repo}
		if line, ok := installer.Installed(dir, tool.Bin); ok {
			e.Installed = true
			e.Version = versionOf(line)
			e.Latest = strings.TrimPrefix(latest[tool.Repo], "v")
			e.Outdated = e.Latest != "" && e.Version != "" && e.Latest != e.Version
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
	nameWidth, stateWidth, outdated := 0, 0, 0
	for _, e := range entries {
		nameWidth = max(nameWidth, len(e.Name))
		stateWidth = max(stateWidth, len(stateOf(e)))
		if e.Outdated {
			outdated++
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
	switch outdated {
	case 0:
	case 1:
		ui.Hint("1 update available, run `facile update`")
	default:
		ui.Hint("%d updates available, run `facile update`", outdated)
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

// versionOf keeps the semver out of a "<bin> <semver>" line.
func versionOf(line string) string {
	if _, ver, found := strings.Cut(line, " "); found {
		return ver
	}
	return line
}
