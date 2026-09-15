package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
	"github.com/FacileStudio/facile/internal/ui"
)

// splitSelf takes facile out of the argument list. It reports whether this run
// should update facile, the arguments that remain, and whether any tool was
// named at all — `facile update facile` must update facile alone, so an
// argument list that held nothing else must not fall back to "every installed
// tool" the way a bare `facile update` does.
func splitSelf(c *cobra.Command, args []string) (self bool, rest []string, named bool) {
	self = len(args) == 0 || updateFlag(c)
	for _, arg := range args {
		if strings.EqualFold(arg, selfTool().Name) {
			self = true
			continue
		}
		rest = append(rest, arg)
	}
	return self, rest, len(args) > 0
}

func updateFlag(c *cobra.Command) bool {
	on, _ := c.Flags().GetBool("all")
	return on
}

// updateTargets reads the catalog after Refresh has written it, so a tool added
// upstream since the last run is selectable now rather than one run later.
func updateTargets(c *cobra.Command, args []string, named bool) ([]manifest.Tool, error) {
	if len(args) > 0 {
		return resolve(args)
	}
	if named {
		return nil, nil
	}
	if updateFlag(c) {
		m, err := catalog()
		if err != nil {
			return nil, err
		}
		return m.Tools, nil
	}
	return installedTools(c)
}

func installedTools(c *cobra.Command) ([]manifest.Tool, error) {
	dir := binDir(c)
	m, err := catalog()
	if err != nil {
		return nil, err
	}
	var tools []manifest.Tool
	for _, tool := range m.Tools {
		if _, ok := installer.Installed(dir, tool.Bin); ok {
			tools = append(tools, tool)
		}
	}
	return tools, nil
}

// unknownTool answers for facile separately now that `facile list` shows it.
// "unknown tool: facile" on a name the listing just printed reads as a bug, and
// the real answer is that facile updates itself but is never installed or
// removed by itself.
func unknownTool(name string) error {
	if strings.EqualFold(name, selfTool().Name) {
		return fmt.Errorf("facile does not install or remove itself — " +
			"`facile update facile` replaces the running binary in place")
	}
	return fmt.Errorf("unknown tool: %s — run `facile list` to see the catalog", name)
}

// updateSelf replaces the running binary at its own path, which is the whole of
// what CLI-STANDARD §3.1 permits: an updater that installs somewhere else is a
// second install. atomicInstall stages beside the destination and renames, so
// overwriting the binary currently executing is safe — that bug is the reason
// this repo exists.
func updateSelf(c *cobra.Command) error {
	dir, ok := selfDir()
	if !ok {
		return brewSelf(c)
	}
	force, _ := c.Flags().GetBool("force")
	if !force {
		if !isSemver(c.Version) {
			ui.Step("facile %s is a source build, leaving it alone", c.Version)
			ui.Hint("facile update facile --force replaces it with the published release")
			return nil
		}
		if _, behind := selfOutdated(selfLatest(), c.Version); !behind {
			ui.Success("facile %s is up to date", c.Version)
			return nil
		}
	}

	ui.Step("Updating facile")
	fromSrc, _ := c.Flags().GetBool("source")
	reported, err := installer.Install(selfTool(), installer.Options{BinDir: dir, FromSrc: fromSrc})
	if err != nil {
		ui.Error("%s", err)
		return fmt.Errorf("facile did not update")
	}
	ui.Success("%s installed to %s", reported, store.Tilde(filepath.Join(dir, selfTool().Bin)))
	return nil
}

// brewSelf answers for a Homebrew-installed facile: report whether it is behind
// and let brew do the upgrade, since overwriting in place would be reverted by
// the next `brew upgrade`.
func brewSelf(c *cobra.Command) error {
	if _, behind := selfOutdated(selfLatest(), c.Version); !behind {
		ui.Success("facile %s is up to date", c.Version)
		return nil
	}
	ui.Step("facile %s is managed by Homebrew", c.Version)
	ui.Hint("%s", upgradeHint())
	return nil
}
