package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

var toolchains = map[string]string{
	"rust": "cargo, install Rust from https://rustup.rs",
	"go":   "go, install Go from https://go.dev/dl",
	"bun":  "bun, install Bun from https://bun.sh",
}

// fromSource builds the tool from a checkout. It clones the requested version
// when one was named, so a --version whose archive cannot be downloaded still
// installs that version rather than whatever main happens to be.
func fromSource(tool manifest.Tool, version, work string) (string, error) {
	hint, known := toolchains[tool.Build]
	if !known {
		return "", fmt.Errorf("unknown build backend: %s", tool.Build)
	}
	name, advice, _ := strings.Cut(hint, ", ")
	if err := need("git", "install git first"); err != nil {
		return "", err
	}
	if err := need(name, advice); err != nil {
		return "", err
	}

	ref := tool.Branch
	if version != "" {
		ref = version
	}

	ui.Step("Fetching source")
	checkout := filepath.Join(work, "src")
	clone := exec.Command("git", "clone", "--depth", "1", "--quiet",
		"--branch", ref, "https://github.com/"+tool.Repo+".git", checkout)
	if out, err := clone.CombinedOutput(); err != nil {
		return "", fmt.Errorf("cannot clone %s at %s: %s", tool.Repo, ref, strings.TrimSpace(string(out)))
	}

	ui.Step("Building from source, this takes a minute")
	src := filepath.Join(checkout, tool.SrcSubdir)
	out := filepath.Join(work, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", err
	}
	switch tool.Build {
	case "rust":
		return buildRust(tool, src, work, out)
	case "go":
		return buildGo(tool, src, checkout, out)
	default:
		return buildBun(tool, src, out)
	}
}

func buildRust(tool manifest.Tool, src, work, out string) (string, error) {
	root := filepath.Join(work, "cargo")
	cmd := exec.Command("cargo", "install", "--path", src, "--root", root, "--force", "--quiet")
	if err := run(cmd); err != nil {
		return "", err
	}
	dest := filepath.Join(out, tool.Bin)
	return dest, os.Rename(filepath.Join(root, "bin", tool.Bin), dest)
}

func buildGo(tool manifest.Tool, src, checkout, out string) (string, error) {
	ldflags := "-s -w"
	if tool.GoVersionVar != "" {
		ldflags += " -X " + tool.GoVersionVar + "=" + describe(checkout)
	}
	dest := filepath.Join(out, tool.Bin)
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", dest, ".")
	cmd.Dir = src
	return dest, run(cmd)
}

func buildBun(tool manifest.Tool, src, out string) (string, error) {
	install := exec.Command("bun", "install", "--frozen-lockfile", "--silent")
	install.Dir = src
	if err := run(install); err != nil {
		return "", err
	}
	compile := exec.Command("bun", "run", "--silent", "build")
	compile.Dir = src
	if err := run(compile); err != nil {
		return "", err
	}
	dest := filepath.Join(out, tool.Bin)
	return dest, os.Rename(filepath.Join(src, tool.Bin), dest)
}

func describe(checkout string) string {
	cmd := exec.Command("git", "describe", "--tags", "--always")
	cmd.Dir = checkout
	out, err := cmd.Output()
	if err != nil {
		return "dev"
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
}

func run(cmd *exec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build failed: %s", lastLine(string(out)))
	}
	return nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

func need(bin, advice string) error {
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("%s not found — %s", bin, advice)
	}
	return nil
}
