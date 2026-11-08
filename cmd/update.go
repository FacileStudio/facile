package cmd

import (
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

// NewUpdateCommand builds the update command and its flags.
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [tool...]",
		Short: "Update installed Facile tools",
		Long: "Bring tools up to their latest release.\n\n" +
			"With no arguments it updates everything already installed. A tool that " +
			"already reports the latest published version is left alone; pass --force " +
			"to reinstall it anyway.",
		RunE: runUpdate,
	}
	cmd.Flags().Bool("all", false, "Update every tool in the catalog")
	cmd.Flags().Bool("no-skill", false, "Skip AI agent skill registration")
	cmd.Flags().Bool("force", false, "Reinstall even when already at the latest release")
	return cmd
}

func runUpdate(c *cobra.Command, args []string) error {
	wantSelf, rest, named := splitSelf(c, args)
	targets, err := updateTargets(c, rest, named)
	if err != nil {
		return err
	}
	if len(targets) == 0 && !wantSelf {
		ui.Step("No Facile tools installed")
		return nil
	}

	force, _ := c.Flags().GetBool("force")
	tools := targets
	if !force {
		tools = stale(c, targets)
	}
	var failure error
	switch {
	case len(tools) > 0:
		failure = installAll(c, tools)
	case len(targets) > 0:
		reportPath(binDir(c))
	}
	if wantSelf {
		if err := updateSelf(c); err != nil && failure == nil {
			failure = err
		}
	}
	return failure
}

var latestTagFn = installer.LatestTag

func stale(c *cobra.Command, tools []manifest.Tool) []manifest.Tool {
	dir := binDir(c)
	checks := checkAll(tools, dir)

	keep := make([]manifest.Tool, 0, len(tools))
	for i, tool := range tools {
		switch {
		case checks[i].err != nil:
			ui.Warn("%s: cannot check for an update (%v), keeping %s", tool.Name, checks[i].err, checks[i].have)
		case checks[i].current:
			ui.Success("%s is up to date", checks[i].have)
		default:
			keep = append(keep, tool)
		}
	}
	return keep
}

type check struct {
	have    string
	current bool
	err     error
}

func checkAll(tools []manifest.Tool, dir string) []check {
	checks := make([]check, len(tools))
	var wg sync.WaitGroup
	for i, tool := range tools {
		wg.Go(func() {
			checks[i] = checkOne(tool, dir)
		})
	}
	wg.Wait()
	return checks
}

// checkOne compares an installed tool with its latest tag. A tool that is not
// installed is always stale. An installed one whose latest tag cannot be read
// is kept as it is: rebuilding it from source because GitHub was slow replaces
// a working binary for no reason.
func checkOne(tool manifest.Tool, dir string) check {
	have, ok := installer.Installed(dir, tool.Bin)
	if !ok {
		return check{}
	}
	tag, err := latestTagFn(tool.Repo)
	if err != nil {
		return check{have: have, err: err}
	}
	return check{have: have, current: matchesTag(have, filepath.Join(dir, tool.Bin), tool, tag)}
}

func upToDate(have, path string, tool manifest.Tool) bool {
	tag, err := latestTagFn(tool.Repo)
	if err != nil {
		return false
	}
	return matchesTag(have, path, tool, tag)
}

func matchesTag(have, path string, tool manifest.Tool, tag string) bool {
	expected := strings.TrimPrefix(tag, "v")

	if have == "" {
		return false
	}
	if v := versionFromPattern(have, path, tool); v != "" {
		return strings.TrimPrefix(v, "v") == expected
	}
	if matchesVersion(have, tool, expected) {
		return true
	}
	installed := versionOf(have)
	if strings.Contains(installed, "dev") {
		return true
	}
	return installed != "" && installed == expected
}

func versionFromPattern(have, path string, tool manifest.Tool) string {
	if tool.VersionDetection.Pattern == "" {
		return ""
	}
	output := have
	if tool.VersionDetection.Cmd != "" {
		out, err := exec.Command(path, strings.Fields(tool.VersionDetection.Cmd)...).Output()
		if err != nil {
			return ""
		}
		output = string(out)
	}
	return manifest.MatchVersion(output, tool.VersionDetection.Pattern)
}

func matchesVersion(have string, tool manifest.Tool, expected string) bool {
	parts := strings.Fields(have)
	if len(parts) >= 2 && parts[0] == tool.Bin && parts[1] == expected {
		return true
	}
	return len(parts) >= 3 && parts[0] == tool.Bin && parts[1] == "version" && parts[2] == expected
}
