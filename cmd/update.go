package cmd

import (
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
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
	installer.Latest(store.LatestPath(), allRepos(targets), !force)

	tools := targets
	if !force {
		tools = stale(c, targets, store.LatestPath())
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

// latestTagFn is a seam for the tests, which must decide what to skip without
// reaching GitHub.
var latestTagFn = installer.LatestTag

// stale drops the tools whose installed binary already reports the latest
// published version, and reports each one it dropped. Resolving a tag is one
// redirect; reinstalling costs an archive, a checksums file, and a rewrite of
// every agent skill on the machine, for a byte-identical binary.
//
// Resolved tags are written back to the version cache as a side effect, so
// `facile list` stays up to date after an update check without waiting for the
// 24h cache TTL.
//
// The checks run concurrently, because ten sequential redirects cost about
// eight seconds on a run that installs nothing. Results land in a slice indexed
// by position rather than a channel, so the report stays in catalog order.
func stale(c *cobra.Command, tools []manifest.Tool, cachePath string) []manifest.Tool {
	dir := binDir(c)
	current, resolved := checkAll(tools, dir)

	if cachePath != "" && len(resolved) > 0 {
		installer.WriteLatest(cachePath, resolved)
	}

	keep := make([]manifest.Tool, 0, len(tools))
	for i, tool := range tools {
		if current[i] != "" {
			ui.Success("%s is up to date", current[i])
			continue
		}
		keep = append(keep, tool)
	}
	return keep
}

// checkAll resolves every tool's installed version and latest tag in parallel.
func checkAll(tools []manifest.Tool, dir string) ([]string, map[string]string) {
	state := &checkState{
		current:  make([]string, len(tools)),
		resolved: make(map[string]string, len(tools)),
	}
	var wg sync.WaitGroup
	for i, tool := range tools {
		wg.Go(func() {
			checkOne(tool, dir, i, state)
		})
	}
	wg.Wait()
	return state.current, state.resolved
}

// checkState is the shared result surface of a concurrent update check: the
// per-tool version, the resolved tag map, and the lock that guards both writes.
type checkState struct {
	current  []string
	resolved map[string]string
	mu       sync.Mutex
}

// checkOne resolves one tool's installed version and its latest tag, so a
// skipped or errant tool never delays the others.
func checkOne(tool manifest.Tool, dir string, i int, state *checkState) {
	if have, ok := installer.Installed(dir, tool.Bin); ok && upToDate(have, tool) {
		state.current[i] = have
	}
	if tool.Repo != "" {
		if tag, err := latestTagFn(tool.Repo); err == nil {
			state.mu.Lock()
			state.resolved[tool.Repo] = tag
			state.mu.Unlock()
		}
	}
}

// upToDate compares the version the installed binary reports against the latest
// release tag. Anything it cannot establish — not installed, no release asset,
// an unreadable tag, a version line that is not a plain semver — is not up to
// date, so the run falls through to the reinstall facile did unconditionally
// before. Skipping is the claim that needs evidence.
//
// Handles version output formats like:
//
//	"{tool} {version}" (standard)
//	"{tool} version {version}" (used by some tools like agenda)
//
// Special case: if the installed version contains "dev" (indicating a
// development or source-built version), we consider it up to date to avoid
// unnecessary rebuild attempts, since rebuilding from source doesn't change the
// functional version unless the source has actually changed.
func upToDate(have string, tool manifest.Tool) bool {
	if have == "" || tool.Asset == "" {
		return false
	}
	tag, err := latestTagFn(tool.Repo)
	if err != nil {
		return false
	}
	expected := strings.TrimPrefix(tag, "v")
	if matchesVersion(have, tool, expected) {
		return true
	}
	installed := versionOf(have)
	if strings.Contains(installed, "dev") {
		return true
	}
	return installed != "" && installed == expected
}

// matchesVersion is true when the reported version line states the tag.
func matchesVersion(have string, tool manifest.Tool, expected string) bool {
	parts := strings.Fields(have)
	if len(parts) >= 2 && parts[0] == tool.Bin && parts[1] == expected {
		return true
	}
	return len(parts) >= 3 && parts[0] == tool.Bin && parts[1] == "version" && parts[2] == expected
}

// allRepos collects every repository the tools belong to.
func allRepos(tools []manifest.Tool) []string {
	repos := make([]string, 0, len(tools))
	for _, tool := range tools {
		repos = append(repos, tool.Repo)
	}
	return repos
}
