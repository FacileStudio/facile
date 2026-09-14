package credstore

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
