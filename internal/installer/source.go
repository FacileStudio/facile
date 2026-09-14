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

	checkout, err := checkoutSource(tool, version, work)
	if err != nil {
		return "", err
	}

	ui.Step("Building from source, this takes a minute")
	out := filepath.Join(work, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", err
	}
	src := filepath.Join(checkout, tool.SrcSubdir)
	return buildFor(tool, src, checkout, work, out)
}

// checkoutSource clones the tool at the requested ref and returns where the
// clone lives.
func checkoutSource(tool manifest.Tool, version, work string) (string, error) {
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
	return checkout, nil
}
