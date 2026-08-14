package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FacileStudio/facile/internal/manifest"
)

// TestRegisterSkillWritesThePiTarget verifies that when the common jardin
// skills directory exists, registerSkill drops the same SKILL.md body there
// under the tool's name — the pi agent's skill source.
func TestRegisterSkillWritesThePiTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// The bridge exists: the pi extension exposes this dir, and we never
	// create it ourselves — its presence is the whole signal.
	skillsDir := filepath.Join(home, ".jardin", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	work := t.TempDir()
	body := []byte("# journal — Facile centralized logging\n\nBinary: `journal`\n")
	integrations := filepath.Join(work, "src", "integrations")
	if err := os.MkdirAll(integrations, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(integrations, "SKILL.md"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	tool := manifest.Tool{Name: "journal", Skill: "journal"}
	registerSkill(tool, work)

	written, err := os.ReadFile(filepath.Join(skillsDir, "journal.md"))
	if err != nil {
		t.Fatalf("pi skill not written: %v", err)
	}
	if string(written) != string(body) {
		t.Fatalf("pi skill content mismatch:\n got %q\nwant %q", string(written), string(body))
	}
}

// TestNoJardinSkillsDirDoesNotCreateOne asserts the pi target is opt-in by
// presence only: a machine without the jardin skills dir gets nothing new on
// disk, so an install stays silent on hosts that never wired the bridge.
func TestNoJardinSkillsDirDoesNotCreateOne(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	work := t.TempDir()
	integrations := filepath.Join(work, "src", "integrations")
	if err := os.MkdirAll(integrations, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(integrations, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := manifest.Tool{Name: "journal", Skill: "journal"}
	registerSkill(tool, work)

	dir := filepath.Join(home, ".jardin")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf(".jardin was created (%v) — a machine without the bridge should stay untouched", err)
	}
}
