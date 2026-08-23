package installer

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

// Options controls a single tool installation.
type Options struct {
	BinDir    string
	Version   string
	FromSrc   bool
	WithSkill bool
}

// Install places one tool's binary in BinDir, preferring a published release
// and falling back to a source build. It returns the version the installed
// binary actually reports — never the version we hoped to install.
func Install(tool manifest.Tool, opts Options) (string, error) {
	if err := prepareBinDir(opts.BinDir); err != nil {
		return "", err
	}
	work, err := os.MkdirTemp("", "facile-")
	if err != nil {
		return "", fmt.Errorf("cannot create a work directory: %w", err)
	}
	defer os.RemoveAll(work)

	if err := missingRequirements(tool); err != nil {
		return "", err
	}

	dest := filepath.Join(opts.BinDir, tool.Bin)
	src, err := build(tool, opts, work)
	if err != nil {
		return "", err
	}
	if err := atomicInstall(src, dest); err != nil {
		return "", err
	}
	if opts.WithSkill {
		registerSkill(tool, work)
	}
	return Verify(dest)
}

// fromReleaseFn is a seam for the tests. TestVerifyChecksumRejectsATamperedArchive
// passed for as long as build swallowed that same error into a source build, so
// the invariant needs a check at the layer that decides, not only at the one that
// detects.
var fromReleaseFn = fromRelease

// build prefers a published release and falls back to a source build, except
// after an integrity failure. A named cause beats a fixed sentence: a private
// repo, a missing platform and a dead network all used to print the same line.
func build(tool manifest.Tool, opts Options, work string) (string, error) {
	if tool.Asset != "" && !opts.FromSrc {
		path, err := fromReleaseFn(tool, opts.Version, work)
		if err == nil {
			return path, nil
		}
		var integrity integrityError
		if errors.As(err, &integrity) {
			return "", err
		}
		ui.Warn("falling back to a source build: %s", err)
	}
	return fromSource(tool, opts.Version, work)
}

// Verify runs the installed binary and returns the single line it reports.
// An installer that claims success without executing the artifact is lying.
func Verify(path string) (string, error) {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("%s was installed but does not run", filepath.Base(path))
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	if line == "" {
		return "", fmt.Errorf("%s reported no version", filepath.Base(path))
	}
	return line, nil
}

// Installed reports the version line of an already-installed tool.
func Installed(binDir, bin string) (string, bool) {
	path := filepath.Join(binDir, bin)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	line, err := Verify(path)
	if err != nil {
		return "", false
	}
	return line, true
}

// Uninstall removes a tool's binary. A tool that is not installed is not an error.
func Uninstall(binDir, bin string) (bool, error) {
	path := filepath.Join(binDir, bin)
	if _, err := os.Stat(path); err != nil {
		return false, nil
	}
	if err := os.Remove(path); err != nil {
		return false, fmt.Errorf("cannot remove %s: %w", path, err)
	}
	return true, nil
}

// atomicInstall stages the new binary beside its destination and renames it into
// place, so replacing a tool while it is running cannot corrupt the running image.
func atomicInstall(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("cannot read the built binary: %w", err)
	}
	staged := filepath.Join(filepath.Dir(dest), fmt.Sprintf(".%s.new.%d", filepath.Base(dest), os.Getpid()))
	if err := os.WriteFile(staged, data, 0o755); err != nil {
		return fmt.Errorf("cannot write %s: %w", staged, err)
	}
	if err := os.Rename(staged, dest); err != nil {
		os.Remove(staged)
		return fmt.Errorf("cannot write %s: %w", dest, err)
	}
	return nil
}

func prepareBinDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", dir, err)
	}
	probe := filepath.Join(dir, ".facile-write-probe")
	if err := os.WriteFile(probe, nil, 0o600); err != nil {
		return fmt.Errorf("%s is not writable", dir)
	}
	return os.Remove(probe)
}

func missingRequirements(tool manifest.Tool) error {
	for _, req := range tool.Requires {
		if _, err := exec.LookPath(req); err != nil {
			return fmt.Errorf("%s needs %s on your PATH — install it first", tool.Name, req)
		}
	}
	return nil
}
