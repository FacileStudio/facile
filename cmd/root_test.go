package cmd

import (
	"reflect"
	"testing"
)

// TestSplitNames tests comma and whitespace splitting across arguments.
func TestSplitNames(t *testing.T) {
	cases := []struct {
		name  string
		input []string
		want  []string
	}{
		{"empty", nil, nil},
		{"single", []string{"poke"}, []string{"poke"}},
		{"multiple args", []string{"poke", "palette"}, []string{"poke", "palette"}},
		{"comma separated single arg", []string{"poke,palette"}, []string{"poke", "palette"}},
		{"comma separated with spaces", []string{"poke, palette"}, []string{"poke", "palette"}},
		{"mixed comma and space args", []string{"poke,palette", "widgets"}, []string{"poke", "palette", "widgets"}},
		{"whitespace within arg", []string{"poke palette"}, []string{"poke", "palette"}},
		{"trailing and leading commas", []string{",poke,", ",palette,"}, []string{"poke", "palette"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := splitNames(c.input)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("splitNames(%v) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}
