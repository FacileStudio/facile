package installer

import (
	"os"
	"path/filepath"

	"github.com/FacileStudio/facile/internal/ui"
)

// writeClaudeSkill writes the skill into the Claude Code skills directory.
func writeClaudeSkill(skill string, body []byte) {
	if !have("claude") {
		return
	}
	dir := filepath.Join(home(), ".claude", "skills", skill)
	if os.MkdirAll(dir, 0o755) == nil &&
		os.WriteFile(filepath.Join(dir, "SKILL.md"), body, 0o644) == nil {
		ui.Success("Claude Code skill installed")
	}
}

// writeCodexSkill writes the skill into the shared Codex AGENTS.md, where the
// tool's own marked section is replaced rather than the whole file overwritten.
func writeCodexSkill(skill string, body []byte) {
	if !have("codex") {
		return
	}
	path := filepath.Join(home(), ".codex", "AGENTS.md")
	if os.MkdirAll(filepath.Dir(path), 0o755) == nil && injectBlock(path, skill, body) == nil {
		ui.Success("Codex skill installed")
	}
}

// writeMyceliumSkill writes the skill into the mycelium skills directory that
// the pi extension exposes.
func writeMyceliumSkill(skill string, body []byte) {
	if !haveMyceliumSkills() {
		return
	}
	dir := filepath.Join(home(), ".mycelium", "skills")
	if os.WriteFile(filepath.Join(dir, skill+".md"), body, 0o644) == nil {
		ui.Success("pi skill installed")
	}
}
