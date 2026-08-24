package cmd

import "testing"

func TestFromHomebrew(t *testing.T) {
	cases := map[string]bool{
		"/opt/homebrew/Caskroom/facile/0.8.0/facile":    true,
		"/usr/local/Cellar/facile/0.8.0/bin/facile":     true,
		"/home/linuxbrew/.linuxbrew/bin/facile":         true,
		"/Users/someone/.local/bin/facile":              false,
		"/usr/local/bin/facile":                         false,
		"/Users/someone/Projects/homebrew-tap/x/facile": false,
	}
	for path, want := range cases {
		if got := fromHomebrew(path); got != want {
			t.Errorf("fromHomebrew(%q) = %v, want %v", path, got, want)
		}
	}
}

// A commit SHA is what a source build reports, and it cannot be ordered against
// a release tag. Claiming it is behind sends the reader to `facile update`,
// which replaces their build with the release.
func TestOutdated(t *testing.T) {
	cases := []struct {
		name         string
		have, latest string
		want         bool
	}{
		{"behind", "0.7.0", "0.8.0", true},
		{"current", "0.8.0", "0.8.0", false},
		{"commit sha", "edf2b6f", "0.25.0", false},
		{"dev build", "dev", "0.8.0", false},
		{"unresolved tag", "0.7.0", "", false},
		{"nothing installed", "", "0.8.0", false},
		{"prerelease", "0.8.0-rc1", "0.8.0", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := outdated(c.have, c.latest); got != c.want {
				t.Errorf("outdated(%q, %q) = %v, want %v", c.have, c.latest, got, c.want)
			}
		})
	}
}

func TestSplitSelf(t *testing.T) {
	original := flagAll
	t.Cleanup(func() { flagAll = original })
	flagAll = false

	t.Run("no arguments takes facile and every installed tool", func(t *testing.T) {
		self, rest, named := splitSelf(nil)
		if !self || len(rest) != 0 || named {
			t.Errorf("got self=%v rest=%v named=%v", self, rest, named)
		}
	})

	t.Run("facile alone updates facile alone", func(t *testing.T) {
		self, rest, named := splitSelf([]string{"facile"})
		if !self || len(rest) != 0 || !named {
			t.Errorf("got self=%v rest=%v named=%v", self, rest, named)
		}
		tools, err := updateTargets(rest, named)
		if err != nil || len(tools) != 0 {
			t.Errorf("naming facile must not widen the run: got %d tools, err %v", len(tools), err)
		}
	})

	t.Run("a named tool leaves facile out", func(t *testing.T) {
		self, rest, named := splitSelf([]string{"nuage"})
		if self || len(rest) != 1 || rest[0] != "nuage" || !named {
			t.Errorf("got self=%v rest=%v named=%v", self, rest, named)
		}
	})

	t.Run("facile alongside a tool takes both", func(t *testing.T) {
		self, rest, _ := splitSelf([]string{"nuage", "facile"})
		if !self || len(rest) != 1 || rest[0] != "nuage" {
			t.Errorf("got self=%v rest=%v", self, rest)
		}
	})
}

func TestUnknownToolAnswersForFacile(t *testing.T) {
	err := unknownTool("facile", nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); got == "unknown tool: facile — run `facile list` to see the catalog" {
		t.Error("facile is a listing row now; saying it is unknown reads as a bug")
	}
}
