package credstore

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// readTOML returns the value for key from the root table of a TOML document,
// or "" when the key is absent or the lookup cannot be parsed. Sub-tables are
// not searched: a root key is what every catalog store uses.
func readTOML(src []byte, key string) (string, error) {
	for line := range strings.SplitSeq(string(src), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			break
		}
		name, value, ok := tomlPair(line)
		if ok && name == key {
			return value, nil
		}
	}
	return "", nil
}

// applyTOML edits the root table line by line. Go has no TOML in its standard
// library, and for a flat config a line editor preserves more than a parser
// would — comments and layout included — without adding a dependency.
func applyTOML(src []byte, sets []field, deletes []string) ([]byte, error) {
	lines := []string{}
	if len(src) > 0 {
		lines = strings.Split(strings.TrimSuffix(string(src), "\n"), "\n")
	}

	editor := newTomlEditor(lines, sets, deletes)
	for _, line := range lines {
		editor.scan(line)
	}

	var added []string
	for _, f := range sets {
		if !editor.done[f.key] {
			added = append(added, fmt.Sprintf("%s = %s", f.key, tomlQuote(f.value)))
		}
	}
	out := append(editor.out[:editor.insertAt], append(added, editor.out[editor.insertAt:]...)...)

	body := strings.Join(out, "\n")
	if body == "" {
		return []byte{}, nil
	}
	return []byte(body + "\n"), nil
}

// tomlEditor walks the lines once, applying deleted and replaced keys in place
// and tracking where newly added keys should be inserted. Starting the first
// `[table]` stops the root-table work: everything from there on is preserved.
type tomlEditor struct {
	sets     []field
	deletes  []string
	out      []string
	done     map[string]bool
	insertAt int
	inRoot   bool
}

func newTomlEditor(lines []string, sets []field, deletes []string) tomlEditor {
	return tomlEditor{
		sets:     sets,
		deletes:  deletes,
		out:      make([]string, 0, len(lines)+len(sets)),
		done:     map[string]bool{},
		insertAt: len(lines),
		inRoot:   true,
	}
}

func (e *tomlEditor) lookup(key string) (string, bool) {
	for _, f := range e.sets {
		if f.key == key {
			return f.value, true
		}
	}
	return "", false
}

func (e *tomlEditor) scan(line string) {
	if strings.HasPrefix(strings.TrimSpace(line), "[") {
		if e.inRoot {
			e.insertAt = len(e.out)
			e.inRoot = false
		}
		e.out = append(e.out, line)
		return
	}
	name, _, ok := tomlPair(line)
	if !ok || !e.inRoot {
		e.out = append(e.out, line)
		return
	}
	if slices.Contains(e.deletes, name) {
		return
	}
	if value, found := e.lookup(name); found {
		e.out = append(e.out, fmt.Sprintf("%s = %s", name, tomlQuote(value)))
		e.done[name] = true
		return
	}
	e.out = append(e.out, line)
}

func tomlPair(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	name, value, ok := strings.Cut(trimmed, "=")
	if !ok {
		return "", "", false
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, " \t") {
		return "", "", false
	}
	value = strings.TrimSpace(value)
	if unquoted, err := strconv.Unquote(value); err == nil {
		value = unquoted
	}
	return name, value, true
}

func tomlQuote(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}
