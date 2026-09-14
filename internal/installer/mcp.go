// mcp.go registers a freshly installed tool's MCP server in the agent harnesses
// on this machine. It is the sibling of skill.go: same shape, same best-effort
// contract, different target. The tool owns the declaration shapes through its
// own install subcommand; facile only decides it should be asked, the same way
// it asks skill.go for a SKILL.md.
package installer

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/FacileStudio/facile/internal/ui"
)

// registerMCP teaches the agent harnesses on this machine about the tool's MCP
// server by running the tool's own install subcommand. Best effort by design,
// like registerSkill: a harness config that refuses (unparseable, or a file
// facile should not touch) must not roll back a working binary, so a failure
// warns and continues.
func registerMCP(dest string) {
	out, err := exec.Command(dest, "install").CombinedOutput()
	if err != nil {
		ui.Warn("could not register %s's MCP server: %s", filepath.Base(dest), strings.TrimSpace(string(out)))
		ui.Hint("run %s install to retry", dest)
	}
}
