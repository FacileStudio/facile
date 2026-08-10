package credstore

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/FacileStudio/facile/internal/manifest"
)

func TestExpand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cases := []struct {
		name string
		xdg  string
		in   string
		want string
	}{
		{"tilde", "", "~/.nuage.yml", filepath.Join(home, ".nuage.yml")},
		{"xdg from env", "/somewhere/cfg", "${xdgConfig}/antenne/config.json", "/somewhere/cfg/antenne/config.json"},
		{"xdg falls back to ~/.config", "", "${xdgConfig}/antenne/config.json", filepath.Join(home, ".config", "antenne", "config.json")},
		{"absolute is left alone", "", "/etc/facile.yml", "/etc/facile.yml"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", c.xdg)
			got, err := Expand(c.in)
			if err != nil {
				t.Fatalf("Expand(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("Expand(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestExpandUserConfigIsPlatformNative guards the distinction that matters:
// ${userConfig} must land where Rust's dirs::config_dir points, which on macOS
// is not ~/.config. Confusing the two writes casier's URL where nobody reads it.
func TestExpandUserConfigIsPlatformNative(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	native, err := os.UserConfigDir()
	if err != nil {
		t.Skip("no platform config directory")
	}
	got, err := Expand("${userConfig}/casier/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(native, "casier", "config.toml"); got != want {
		t.Fatalf("Expand = %q, want %q", got, want)
	}
	if runtime.GOOS == "darwin" && !strings.Contains(got, filepath.Join("Library", "Application Support")) {
		t.Fatalf("on macOS ${userConfig} must be Library/Application Support, got %q", got)
	}
}

func TestWritePreservesUnknownKeys(t *testing.T) {
	cases := []struct {
		name     string
		file     string
		format   string
		existing string
		contains []string
	}{
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

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
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
		})
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

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("credential file is %o, want 600", got)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("credential directory is %o, want 700", got)
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
