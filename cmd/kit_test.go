package cmd

import (
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

type kitArgsCase struct {
	name string
	in   []string
	from string
	want []string
	ok   bool
}

func kitArgsCases() []kitArgsCase {
	return []kitArgsCase{
		{"install", []string{"install", "opus", "--source", "--version", "v1.2.0"}, "install",
			[]string{"add", "opus", "--source", "--version", "v1.2.0"}, true},
		{"add alias", []string{"add", "opus", "--no-skill"}, "add", []string{"add", "opus", "--no-skill"}, true},
		{"uninstall", []string{"uninstall", "opus"}, "uninstall", []string{"del", "opus"}, true},
		{"rm alias", []string{"rm", "opus"}, "rm", []string{"del", "opus"}, true},
		{"update", []string{"update", "--all", "--force", "--check"}, "update",
			[]string{"update", "--all", "--force", "--check"}, true},
		{"list", []string{"list", "--json"}, "list", []string{"list", "--json"}, true},
		{"root flag before", []string{"--bin-dir", "/opt/bin", "install", "opus"}, "install",
			[]string{"add", "--bin-dir", "/opt/bin", "opus"}, true},
		{"root flag with equals", []string{"--bin-dir=/opt/bin", "list"}, "list",
			[]string{"list", "--bin-dir=/opt/bin"}, true},
		{"login stays", []string{"login", "opus"}, "", nil, false},
		{"doctor stays", []string{"doctor"}, "", nil, false},
		{"self-update stays", []string{"update", "facile"}, "", nil, false},
		{"no command", []string{"--version"}, "", nil, false},
		{"empty", nil, "", nil, false},
	}
}

// TestKitArgs pins the facile to kit command mapping and checks that flags
// pass through unchanged and in order.
func TestKitArgs(t *testing.T) {
	for _, c := range kitArgsCases() {
		t.Run(c.name, func(t *testing.T) {
			from, got, ok := kitArgs(c.in)
			if from != c.from || ok != c.ok || !reflect.DeepEqual(got, c.want) {
				t.Errorf("kitArgs(%v) = %q, %v, %v, want %q, %v, %v", c.in, from, got, ok, c.from, c.want, c.ok)
			}
		})
	}
}

// TestForwardToKitAbsent checks that without kit on PATH facile prints how to
// get it on stderr and returns, so the old command still runs.
func TestForwardToKitAbsent(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = write
	forwardToKit([]string{"install", "opus"}, func(string) (string, error) {
		return "", errors.New("not found")
	})
	os.Stderr = saved
	write.Close()
	out, _ := io.ReadAll(read)
	if lines := strings.Count(string(out), "\n"); lines != 1 {
		t.Fatalf("want one stderr line, got %d: %q", lines, out)
	}
	if !strings.Contains(string(out), kitInstallHint) {
		t.Errorf("stderr %q does not name %q", out, kitInstallHint)
	}
}
