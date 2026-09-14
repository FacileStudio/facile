package credstore

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/FacileStudio/facile/internal/manifest"
)

func TestWritePreservesUnknownKeys(t *testing.T) {
	for _, c := range writePreserveCases() {
		t.Run(c.name, func(t *testing.T) {
			assertWritePreserves(t, c)
		})
	}
}

type writePreserveCase struct {
	name     string
	file     string
	format   string
	existing string
	contains []string
}

func writePreserveCases() []writePreserveCase {
	return []writePreserveCase{
		{
			name:   "yaml keeps nuage's sync settings",
			file:   ".nuage.yml",
			format: "yaml",
			existing: "server_url: http://old\ntoken: OLD\nsync_dir: ~/Nuage\npoll_interval: 30\n" +
				"ignore_patterns:\n  - .git\n  - node_modules\n",
			contains: []string{"sync_dir: ~/Nuage", "poll_interval: 30", "- node_modules", "token: NEW", "server_url: http://new"},
		},
		{
			name:     "json keeps antenne's unknown fields",
			file:     "config.json",
			format:   "json",
			existing: "{\n  \"url\": \"http://old\",\n  \"token\": \"OLD\",\n  \"theme\": \"dark\"\n}\n",
			contains: []string{"\"theme\": \"dark\"", "\"token\": \"NEW\"", "\"url\": \"http://new\""},
		},
		{
			name:     "toml keeps comments and other tables",
			file:     "config.toml",
			format:   "toml",
			existing: "# hand written\nserver_url = \"http://old\"\ntoken = \"OLD\"\n\n[ui]\ncolor = true\n",
			contains: []string{"# hand written", "[ui]", "color = true", "token = \"NEW\"", "server_url = \"http://new\""},
		},
	}
}

// assertWritePreserves writes a fresh credential over the sample file and
// checks that the fields the tool keeps elsewhere all survived.
func assertWritePreserves(t *testing.T, c writePreserveCase) {
	dir := t.TempDir()
	path := filepath.Join(dir, c.file)
	if err := os.WriteFile(path, []byte(c.existing), 0o600); err != nil {
		t.Fatal(err)
	}

	s := &manifest.Store{
		Kind:       "file",
		Path:       path,
		Format:     c.format,
		TokenField: "token",
		URLField:   urlFieldFor(c.format),
		Mode:       0o600,
		Preserve:   true,
	}
	if _, err := Write(s, Credential{Token: "NEW", ServerURL: "http://new"}); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, want := range c.contains {
		if !strings.Contains(body, want) {
			t.Fatalf("rewritten file lost %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "OLD") {
		t.Fatalf("the old credential survived the rewrite:\n%s", body)
	}
}

// urlFieldFor mirrors the catalog: antenne's JSON calls it url, everybody
// else's calls it server_url.
func urlFieldFor(format string) string {
	if format == "json" {
		return "url"
	}
	return "server_url"
}

func TestWriteCreatesFileAtItsTargetMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no mode bits on Windows")
	}

	dir := filepath.Join(t.TempDir(), "antenne")
	path := filepath.Join(dir, "config.json")
	s := &manifest.Store{
		Kind:       "file",
		Path:       path,
		Format:     "json",
		TokenField: "token",
		URLField:   "url",
		Mode:       0o600,
		DirMode:    0o700,
		Preserve:   true,
	}
	if _, err := Write(s, Credential{Token: "secret", ServerURL: "http://localhost:9090"}); err != nil {
		t.Fatal(err)
	}

	assertPerm(t, path, "credential file", 0o600)
	assertPerm(t, dir, "credential directory", 0o700)
}

// assertPerm checks that a path was left at the given permission bits.
func assertPerm(t *testing.T, path, what string, want fs.FileMode) {
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s is %o, want %o", what, got, want)
	}
}

func TestWriteTightensAnAlreadyLooseFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no mode bits on Windows")
	}

	path := filepath.Join(t.TempDir(), ".nuage.yml")
	if err := os.WriteFile(path, []byte("server_url: http://old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &manifest.Store{
		Kind: "file", Path: path, Format: "yaml",
		TokenField: "token", URLField: "server_url", Mode: 0o600, Preserve: true,
	}
	if _, err := Write(s, Credential{Token: "secret", ServerURL: "http://new"}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("credential file is %o, want 600", got)
	}
}

func TestWriteStoresExtraFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mycelium.yml")
	s := &manifest.Store{
		Kind: "file", Path: path, Format: "yaml",
		TokenField: "token", URLField: "url", Mode: 0o600, Preserve: true,
		Extra: []string{"machine"},
	}
	cred := Credential{Token: "t", ServerURL: "http://j", Extra: map[string]string{"machine": "lucy"}}
	if _, err := Write(s, cred); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "machine: lucy") {
		t.Fatalf("the machine field was not stored:\n%s", raw)
	}
}

func TestClearKeepsTheServerURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mycelium.yml")
	if err := os.WriteFile(path, []byte("url: http://j\ntoken: SECRET\nspace: abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := &manifest.Store{
		Kind: "file", Path: path, Format: "yaml",
		TokenField: "token", URLField: "url", Mode: 0o600, Preserve: true,
	}
	if _, err := Clear(s, "http://j"); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(path)
	body := string(raw)
	if strings.Contains(body, "SECRET") {
		t.Fatalf("the credential survived the logout:\n%s", body)
	}
	for _, want := range []string{"url: http://j", "space: abc"} {
		if !strings.Contains(body, want) {
			t.Fatalf("logout dropped %q:\n%s", want, body)
		}
	}
	if got := StoredServerURL(s); got != "http://j" {
		t.Fatalf("StoredServerURL = %q, want http://j", got)
	}
}
