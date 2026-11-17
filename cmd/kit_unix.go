//go:build unix

package cmd

import (
	"os"
	"syscall"

	"github.com/FacileStudio/facile/internal/ui"
)

// runKit replaces the facile process with kit, so kit owns the terminal, the
// signals and the exit code. It returns only when the exec itself fails.
func runKit(path string, args []string) {
	err := syscall.Exec(path, append([]string{"kit"}, args...), os.Environ())
	ui.Error("cannot run kit at %s: %s", path, err)
	os.Exit(1)
}
