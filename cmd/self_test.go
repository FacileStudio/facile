package cmd

import "testing"

func TestFromHomebrew(t *testing.T) {
	cases := map[string]bool{
		"/opt/homebrew/Caskroom/facile/0.7.0/facile":    true,
		"/usr/local/Cellar/facile/0.7.0/bin/facile":     true,
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

func TestSelfOutdated(t *testing.T) {
	latest := map[string]string{facileRepo: "v0.8.0"}
	original := version
	t.Cleanup(func() { version = original })

	version = "0.7.0"
	if tag, out := selfOutdated(latest); !out || tag != "0.8.0" {
		t.Errorf("behind: got %q %v, want 0.8.0 true", tag, out)
	}

	version = "0.8.0"
	if _, out := selfOutdated(latest); out {
		t.Error("current build reported as outdated")
	}

	version = "dev"
	if _, out := selfOutdated(latest); out {
		t.Error("a source build has no tag to compare against and must never be called outdated")
	}

	version = "0.7.0"
	if _, out := selfOutdated(nil); out {
		t.Error("an unresolved tag must not be treated as an upgrade")
	}
}
