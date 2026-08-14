package installer

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

// registerSkill teaches the AI coding agents on this machine about the tool.
// It is best effort by design: a missing skill file never fails an install.
func registerSkill(tool manifest.Tool, work string) {
	if tool.Skill == "" {
		return
	}
	if !have("claude") && !have("codex") && !haveMyceliumSkills() {
		return
	}
	body, err := skillBody(tool, work)
	if err != nil || len(body) == 0 {
		return
	}
	if have("claude") {
		dir := filepath.Join(home(), ".claude", "skills", tool.Skill)
		if os.MkdirAll(dir, 0o755) == nil &&
			os.WriteFile(filepath.Join(dir, "SKILL.md"), body, 0o644) == nil {
			ui.Success("Claude Code skill installed")
		}
	}
	if have("codex") {
		path := filepath.Join(home(), ".codex", "AGENTS.md")
		if os.MkdirAll(filepath.Dir(path), 0o755) == nil && injectBlock(path, tool.Skill, body) == nil {
			ui.Success("Codex skill installed")
		}
	}
	if haveMyceliumSkills() {
		// Pi (and any agent wired through the mycelium extension) reads root
		// .md files from the mycelium skills dir. Write the same body there,
		// named by the tool, so one integrations/SKILL.md feeds every agent.
		dir := filepath.Join(home(), ".mycelium", "skills")
		if os.WriteFile(filepath.Join(dir, tool.Skill+".md"), body, 0o644) == nil {
			ui.Success("pi skill installed")
		}
	}
}

// haveMyceliumSkills reports whether this machine runs the shared mycelium
// skills directory that the pi extension exposes. It is the one skill target
// we deliberately do not create: writing an agent directory on a machine that
// never set the bridge up would be noise. The dir's presence is the signal.
func haveMyceliumSkills() bool {
	info, err := os.Stat(filepath.Join(home(), ".mycelium", "skills"))
	return err == nil && info.IsDir()
}

func skillBody(tool manifest.Tool, work string) ([]byte, error) {
	local := filepath.Join(work, "src", "integrations", "SKILL.md")
	if body, err := os.ReadFile(local); err == nil {
		return body, nil
	}
	return get(fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/integrations/SKILL.md",
		tool.Repo, tool.Branch))
}

// injectBlock replaces the tool's marked section in a shared agent file,
// preserving everything the user wrote outside the markers.
func injectBlock(path, skill string, body []byte) error {
	start := "<!-- " + skill + ":start -->"
	end := "<!-- " + skill + ":end -->"

	var kept []string
	if existing, err := os.Open(path); err == nil {
		scanner := bufio.NewScanner(existing)
		skipping := false
		for scanner.Scan() {
			switch line := scanner.Text(); {
			case line == start:
				skipping = true
			case line == end:
				skipping = false
			case !skipping:
				kept = append(kept, line)
			}
		}
		existing.Close()
	}

	var out strings.Builder
	if len(kept) > 0 {
		out.WriteString(strings.TrimRight(strings.Join(kept, "\n"), "\n") + "\n\n")
	}
	out.WriteString(start + "\n")
	out.Write(body)
	if !strings.HasSuffix(string(body), "\n") {
		out.WriteString("\n")
	}
	out.WriteString(end + "\n")
	return os.WriteFile(path, []byte(out.String()), 0o644)
}

func have(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func home() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return dir
}
