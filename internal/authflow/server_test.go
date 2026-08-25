package authflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FacileStudio/facile/internal/manifest"
)

// storedAuth is a tool that has been logged in to before: a catalog default, an
// environment variable and a config file that already names a server.
func storedAuth(t *testing.T) *manifest.Auth {
	t.Helper()
	stored := filepath.Join(os.Getenv("HOME"), ".stored.yml")
	if err := os.WriteFile(stored, []byte("server_url: http://from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &manifest.Auth{
		DefaultServerURL: "https://default.example",
		Env:              manifest.Env{EnvURL: "TEST_SERVER_URL"},
		Store: &manifest.Store{
			Kind: "file", Path: stored, Format: "yaml", URLField: "server_url",
		},
	}
}

func TestResolveServerPrecedence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cases := []struct {
		name string
		flag string
		env  string
		want string
	}{
		{"the flag wins", "http://from-flag", "http://from-env", "http://from-flag"},
		{"then the environment", "", "http://from-env", "http://from-env"},
		{"then what a previous login stored", "", "", "http://from-file"},
		{"a bare host gets https", "example.test", "", "https://example.test"},
		{"a trailing slash is dropped", "http://x/", "", "http://x"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("TEST_SERVER_URL", c.env)
			got, err := resolveServer(storedAuth(t), c.flag)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("resolveServer = %q, want %q", got, c.want)
			}
		})
	}
}

// The catalog default is last, and only reached when nothing else answered.
func TestResolveServerFallsBackToTheCatalogDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TEST_SERVER_URL", "")

	a := storedAuth(t)
	a.Store.Path = filepath.Join(home, "absent.yml")

	got, err := resolveServer(a, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://default.example" {
		t.Fatalf("resolveServer = %q, want the catalog default", got)
	}
}
