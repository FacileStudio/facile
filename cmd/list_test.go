package cmd

import "testing"

func TestStateOf(t *testing.T) {
	cases := []struct {
		name string
		in   entry
		want string
	}{
		{"outdated", entry{Installed: true, Version: "1.2.3", Latest: "1.3.0", Outdated: true}, "1.2.3 → 1.3.0"},
		{"current", entry{Installed: true, Version: "1.2.3", Latest: "1.2.3"}, "1.2.3"},
		{"absent", entry{}, "not installed"},
		{"unknown latest", entry{Installed: true, Version: "1.2.3"}, "1.2.3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stateOf(c.in); got != c.want {
				t.Errorf("stateOf = %q, want %q", got, c.want)
			}
		})
	}
}
