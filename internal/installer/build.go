package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/FacileStudio/facile/internal/manifest"
)

// buildFor hands the build off to the toolchain the tool declares.
func buildFor(tool manifest.Tool, src, checkout, work, out string) (string, error) {
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

func run(cmd *exec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err != nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		return fmt.Errorf("build failed: %s", lines[len(lines)-1])
	}
	return nil
}

func need(bin, advice string) error {
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("%s not found — %s", bin, advice)
	}
	return nil
}
