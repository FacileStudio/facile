package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/facile/internal/installer"
	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
	"github.com/FacileStudio/facile/internal/ui"
)

var (
	flagForce bool
)

var updateCmd = &cobra.Command{
	Use:   "update [tool...]",
	Short: "Update installed Facile tools",
	Long: "Bring tools up to their latest release.\n\n" +
		"With no arguments it updates everything already installed. A tool that " +
		"already reports the latest published version is left alone; pass --force " +
		"to reinstall it anyway.",
	RunE: func(_ *cobra.Command, args []string) error {
		// catalog() refreshes the catalog before reading it, so a tool added
		// upstream since the last run is selectable now rather than one run
		// later. The version cache is refreshed below for the targets that
		// matter; the rest can wait for the next `facile list`.
		wantSelf, rest, named := splitSelf(args)
		targets, err := updateTargets(rest, named)
		if err != nil {
			return err
		}
		if len(targets) == 0 && !wantSelf {
			ui.Step("No Facile tools installed")
			return nil
		}

		// Resolve the latest published tags and write them to the version cache,
		// so `facile list` shows the freshest info without waiting for the 24h TTL.
		_ = installer.Latest(store.LatestPath(), allRepos(targets), !flagForce)

		tools := targets
		if !flagForce {
			tools = stale(targets, store.LatestPath())
		}
		var failure error
		switch {
		case len(tools) > 0:
			failure = installAll(tools)
		case len(targets) > 0:
			reportPath(binDir())
		}
		if wantSelf {
			if err := updateSelf(); err != nil && failure == nil {
				failure = err
			}
		}
		return failure
	},
}

func init() {
	updateCmd.Flags().BoolVar(&flagAll, "all", false, "Update every tool in the catalog")
	updateCmd.Flags().BoolVar(&flagNoSkill, "no-skill", false, "Skip AI agent skill registration")
	updateCmd.Flags().BoolVar(&flagForce, "force", false, "Reinstall even when already at the latest release")
	rootCmd.AddCommand(updateCmd)
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
func stale(tools []manifest.Tool, cachePath string) []manifest.Tool {
	dir := binDir()
	current := make([]string, len(tools))
	resolved := make(map[string]string, len(tools))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i, tool := range tools {
		wg.Go(func() {
			if have, ok := installer.Installed(dir, tool.Bin); ok && upToDate(have, tool) {
				current[i] = have
			}
			if tool.Repo != "" {
				if tag, err := latestTagFn(tool.Repo); err == nil {
					mu.Lock()
					resolved[tool.Repo] = tag
					mu.Unlock()
				}
			}
		})
	}
	wg.Wait()

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

// upToDate compares the version the installed binary reports against the latest
// release tag. Anything it cannot establish — not installed, no release asset,
// an unreadable tag, a version line that is not a plain semver — is not up to
// date, so the run falls through to the reinstall facile did unconditionally
// before. Skipping is the claim that needs evidence.
//
// Handles version output formats like:
//   "{tool} {version}" (standard)
//   "{tool} version {version}" (used by some tools like agenda)
// 
// Special case: if the installed version contains "dev" (indicating a development
// or source-built version), we consider it up to date to avoid unnecessary
// rebuild attempts, since rebuilding from source doesn't change the functional
// version unless the source has actually changed.
func upToDate(have string, tool manifest.Tool) bool {
	if have == "" || tool.Asset == "" {
		return false
	}
	tag, err := latestTagFn(tool.Repo)
	if err != nil {
		return false
	}
	expected := strings.TrimPrefix(tag, "v")
	
	// Extract version from the installed binary output
	// Try to handle formats like:
	//   "{tool} {version}"
	//   "{tool} version {version}"
	parts := strings.Fields(have)
	if len(parts) >= 2 {
		// Standard format: "{tool} {version}"
		if parts[0] == tool.Bin {
			if parts[1] == expected {
				return true
			}
		}
		// Format with "version": "{tool} version {version}"
		if len(parts) >= 3 && parts[0] == tool.Bin && parts[1] == "version" {
			if parts[2] == expected {
				return true
			}
		}
	}
	
	// Fall back to original logic for backwards compatibility
	installed := versionOf(have)
	// Special case: consider development versions up to date to avoid unnecessary rebuilds
	if strings.Contains(installed, "dev") {
		return true
	}
	return installed != "" && installed == expected
}

// splitSelf takes facile out of the argument list. It reports whether this run
// should update facile, the arguments that remain, and whether any tool was
// named at all — `facile update facile` must update facile alone, so an
// argument list that held nothing else must not fall back to "every installed
// tool" the way a bare `facile update` does.
func splitSelf(args []string) (self bool, rest []string, named bool) {
	self = len(args) == 0 || flagAll
	for _, arg := range args {
		if strings.EqualFold(arg, selfTool().Name) {
			self = true
			continue
		}
		rest = append(rest, arg)
	}
	return self, rest, len(args) > 0
}

// updateTargets reads the catalog after Refresh has written it, so a tool added
// upstream since the last run is selectable now rather than one run later.
func updateTargets(args []string, named bool) ([]manifest.Tool, error) {
	if len(args) > 0 {
		return resolve(args)
	}
	if named {
		return nil, nil
	}
	if flagAll {
		return catalog().Tools, nil
	}
	return installedTools(), nil
}

// updateSelf replaces the running binary at its own path, which is the whole of
// what CLI-STANDARD §3.1 permits: an updater that installs somewhere else is a
// second install. atomicInstall stages beside the destination and renames, so
// overwriting the binary currently executing is safe — that bug is the reason
// this repo exists.
func updateSelf() error {
	dir, ok := selfDir()
	if !ok {
		// Resolve before mentioning brew. Returning early left the version cache
		// untouched, so a Homebrew user was told to upgrade on every run and the
		// listing kept naming whichever tag the cache happened to hold.
		if _, behind := selfOutdated(selfLatest(true)); !behind {
			ui.Success("facile %s is up to date", version)
			return nil
		}
		ui.Step("facile %s is managed by Homebrew", version)
		ui.Hint("%s", upgradeHint())
		return nil
	}
	if !flagForce {
		if !isSemver(version) {
			ui.Step("facile %s is a source build, leaving it alone", version)
			ui.Hint("facile update facile --force replaces it with the published release")
			return nil
		}
		if _, behind := selfOutdated(selfLatest(true)); !behind {
			ui.Success("facile %s is up to date", version)
			return nil
		}
	}

	ui.Step("Updating facile")
	reported, err := installer.Install(selfTool(), installer.Options{BinDir: dir, FromSrc: flagSource})
	if err != nil {
		ui.Error("%s", err)
		return fmt.Errorf("facile did not update")
	}
	ui.Success("%s installed to %s", reported, store.Tilde(filepath.Join(dir, selfTool().Bin)))
	return nil
}

func installedTools() []manifest.Tool {
	dir := binDir()
	var tools []manifest.Tool
	for _, tool := range catalog().Tools {
		if _, ok := installer.Installed(dir, tool.Bin); ok {
			tools = append(tools, tool)
		}
	}
	return tools
}

// unknownTool answers for facile separately now that `facile list` shows it.
// "unknown tool: facile" on a name the listing just printed reads as a bug, and
// the real answer is that facile updates itself but is never installed or
// removed by itself.
func unknownTool(name string, m *manifest.Manifest) error {
	if strings.EqualFold(name, selfTool().Name) {
		return fmt.Errorf("facile does not install or remove itself — " +
			"`facile update facile` replaces the running binary in place")
	}
	return fmt.Errorf("unknown tool: %s — run `facile list` to see the catalog", name)
}

// allRepos collects every repository the tools belong to.
func allRepos(tools []manifest.Tool) []string {
	repos := make([]string, 0, len(tools))
	for _, tool := range tools {
		repos = append(repos, tool.Repo)
	}
	return repos
}
