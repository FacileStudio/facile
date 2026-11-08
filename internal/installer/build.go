package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/FacileStudio/facile/internal/manifest"
)

// buildFor hands the build off to the toolchain the tool declares. Each
// backend returns one path per binary, in BinList order.
func buildFor(tool manifest.Tool, src, checkout, out string) ([]string, error) {
	switch tool.Build {
	case "rust":
		return buildRust(tool, src, checkout, out)
	case "go":
		return buildGo(tool, src, checkout, out)
	default:
		return buildBun(tool, src, checkout, out)
	}
}

// buildRust compiles the workspace or package and collects every declared
// binary from the shared target directory, so one repository installs all
// of its widgets in one pass.
func buildRust(tool manifest.Tool, src, checkout, out string) ([]string, error) {
	cmd := exec.Command("cargo", "build", "--release", "--quiet")
	cmd.Dir = src
	cmd.Env = ignoreRepoMise(checkout)
	if err := run(cmd); err != nil {
		return nil, err
	}
	dests := make([]string, 0, len(tool.BinList()))
	for _, bin := range tool.BinList() {
		dest := filepath.Join(out, bin)
		if err := os.Rename(filepath.Join(src, "target", "release", bin), dest); err != nil {
			return nil, fmt.Errorf("cargo finished but %s is missing: %w", bin, err)
		}
		dests = append(dests, dest)
	}
	return dests, nil
}

func buildGo(tool manifest.Tool, src, checkout, out string) ([]string, error) {
	bins := tool.BinList()
	if len(bins) > 1 {
		return nil, fmt.Errorf("the go backend builds one binary from %s — split the tool or publish a release asset", tool.SrcSubdir)
	}
	ldflags := "-s -w"
	if tool.GoVersionVar != "" {
		ldflags += " -X " + tool.GoVersionVar + "=" + describe(checkout)
	}
	dest := filepath.Join(out, bins[0])
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", dest, ".")
	cmd.Dir = src
	cmd.Env = ignoreRepoMise(checkout)
	return []string{dest}, run(cmd)
}

func buildBun(tool manifest.Tool, src, checkout, out string) ([]string, error) {
	install := exec.Command("bun", "install", "--frozen-lockfile", "--silent")
	install.Dir = src
	install.Env = ignoreRepoMise(checkout)
	if err := run(install); err != nil {
		return nil, err
	}
	compile := exec.Command("bun", "run", "--silent", "build")
	compile.Dir = src
	compile.Env = ignoreRepoMise(checkout)
	if err := run(compile); err != nil {
		return nil, err
	}
	dests := make([]string, 0, len(tool.BinList()))
	for _, bin := range tool.BinList() {
		dest := filepath.Join(out, bin)
		if err := os.Rename(filepath.Join(src, bin), dest); err != nil {
			return nil, fmt.Errorf("bun finished but %s is missing: %w", bin, err)
		}
		dests = append(dests, dest)
	}
	return dests, nil
}

// describe names the checkout as a tag, falling back to a marker when the clone
// is not shallow enough for git describe to answer.
func describe(checkout string) string {
	cmd := exec.Command("git", "describe", "--tags", "--always")
	cmd.Dir = checkout
	out, err := cmd.Output()
	if err != nil {
		return "dev"
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
}

// maxFailureLines bounds how much of a failed build's output is reported. Build
// tools print the errors first and a one-line summary last, so the tail carries
// the cause; the summary alone ("could not compile `sidebar`", with no reason)
// leaves the user nothing to act on.
const maxFailureLines = 20

func run(cmd *exec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if detail := lastLines(string(out), maxFailureLines); detail != "" {
		return fmt.Errorf("build failed:\n%s", detail)
	}
	return fmt.Errorf("build failed: %w", err)
}

// lastLines keeps at most n trailing lines of a build log.
func lastLines(out string, n int) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func need(bin, advice string) error {
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("%s not found — %s", bin, advice)
	}
	return nil
}

// ignoreRepoMise keeps mise shims from reading the cloned repository's own
// mise.toml. An untrusted config makes every shimmed toolchain abort, and
// trusting a freshly cloned file would run its hooks, so the build uses the
// toolchain the person already installed instead.
func ignoreRepoMise(dir string) []string {
	paths := dir
	if existing := os.Getenv("MISE_IGNORED_CONFIG_PATHS"); existing != "" {
		paths = existing + string(os.PathListSeparator) + dir
	}
	return append(os.Environ(), "MISE_IGNORED_CONFIG_PATHS="+paths)
}
