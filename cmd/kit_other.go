//go:build !unix

package cmd

import (
	"errors"
	"os"
	"os/exec"

	"github.com/FacileStudio/facile/internal/ui"
)

// runKit runs kit as a child where exec is not available and exits with its
// exit code, so a script sees the same status it would from kit directly.
func runKit(path string, args []string) {
	cmd := exec.Command(path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		os.Exit(0)
	case errors.As(err, &exit):
		os.Exit(exit.ExitCode())
	}
	ui.Error("cannot run kit at %s: %s", path, err)
	os.Exit(1)
}
