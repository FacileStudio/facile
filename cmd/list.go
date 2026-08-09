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

var (
	flagJSON  bool
	flagQuiet bool
)

type entry struct {
	Name      string `json:"name"`
	Summary   string `json:"summary"`
	Repo      string `json:"repo"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List the Facile tools and what is installed",
	RunE: func(_ *cobra.Command, _ []string) error {
		entries := survey(catalog())
		switch {
		case flagJSON:
			return json.NewEncoder(os.Stdout).Encode(entries)
		case flagQuiet:
			printNames(entries)
		default:
			printTable(entries)
		}
		return nil
	},
}

func init() {
	listCmd.Flags().BoolVar(&flagJSON, "json", false, "Print one JSON document to stdout")
	listCmd.Flags().BoolVarP(&flagQuiet, "quiet", "q", false, "Print installed tool names only")
	rootCmd.AddCommand(listCmd)
}

func survey(m *manifest.Manifest) []entry {
	dir := binDir()
	entries := make([]entry, 0, len(m.Tools))
	for _, tool := range m.Tools {
		e := entry{Name: tool.Name, Summary: tool.Summary, Repo: tool.Repo}
		if line, ok := installer.Installed(dir, tool.Bin); ok {
			e.Installed = true
			e.Version = versionOf(line)
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

func printTable(entries []entry) {
	width := 0
	for _, e := range entries {
		if len(e.Name) > width {
			width = len(e.Name)
		}
	}
	for _, e := range entries {
		state := ui.Dim("not installed")
		if e.Installed {
			state = e.Version
		}
		fmt.Printf("%-*s  %-10s  %s\n", width, e.Name, state, ui.Dim(e.Summary))
	}
}

// versionOf keeps the semver out of a "<bin> <semver>" line.
func versionOf(line string) string {
	if _, ver, found := strings.Cut(line, " "); found {
		return ver
	}
	return line
}
