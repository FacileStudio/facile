package credstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
)

const (
	defaultMode    fs.FileMode = 0o600
	defaultDirMode fs.FileMode = 0o700
)

// writeFile applies sets and deletes to the tool's config file and returns the
// path it wrote. When the store is marked preserve, the existing file is read
// first: nuage keeps sync_dir and ignore_patterns in the same file as its
// token, and a wholesale overwrite silently resets a user's sync directory.
func writeFile(s *manifest.Store, sets []field, deletes []string) (string, error) {
	path, err := Expand(s.Path)
	if err != nil {
		return "", err
	}

	mode := pickMode(s.Mode, defaultMode)
	if err := os.MkdirAll(filepath.Dir(path), pickMode(s.DirMode, defaultDirMode)); err != nil {
		return "", fmt.Errorf("cannot create %s — check the directory's permissions", store.Tilde(filepath.Dir(path)))
	}

	var existing []byte
	if s.Preserve {
		if raw, err := os.ReadFile(path); err == nil {
			existing = raw
		}
	}

	body, err := apply(s.Format, existing, sets, deletes)
	if err != nil {
		return "", fmt.Errorf("cannot update %s — %s", store.Tilde(path), err)
	}
	if err := createAt(path, body, mode); err != nil {
		return "", fmt.Errorf("cannot write %s — check the file's permissions", store.Tilde(path))
	}
	return path, nil
}

// createAt opens the file at its final mode rather than writing it and
// chmodding afterwards, which would leave a window where a credential sits at
// 0644. The explicit Chmod only matters for a file that already existed, whose
// mode the open flags do not touch.
func createAt(path string, body []byte, mode fs.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := file.Chmod(mode); err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		return err
	}
	return file.Sync()
}

func pickMode(raw uint32, fallback fs.FileMode) fs.FileMode {
	if raw == 0 {
		return fallback
	}
	return fs.FileMode(raw)
}

func apply(format string, src []byte, sets []field, deletes []string) ([]byte, error) {
	switch format {
	case "yaml", "yml":
		return applyYAML(src, sets, deletes)
	case "json":
		return applyJSON(src, sets, deletes)
	case "toml":
		return applyTOML(src, sets, deletes)
	default:
		return nil, fmt.Errorf("unknown config format %q", format)
	}
}

func read(format string, src []byte, key string) (string, error) {
	switch format {
	case "yaml", "yml":
		var doc yaml.MapSlice
		if err := yaml.UnmarshalWithOptions(src, &doc, yaml.UseOrderedMap()); err != nil {
			return "", err
		}
		for _, item := range doc {
			if fmt.Sprint(item.Key) == key {
				return fmt.Sprint(item.Value), nil
			}
		}
	case "json":
		items, err := decodeJSON(src)
		if err != nil {
			return "", err
		}
		for _, item := range items {
			if item.key == key {
				var value string
				if err := json.Unmarshal(item.raw, &value); err != nil {
					return "", err
				}
				return value, nil
			}
		}
	case "toml":
		for _, line := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "[") {
				break
			}
			name, value, ok := tomlPair(line)
			if ok && name == key {
				return value, nil
			}
		}
	}
	return "", nil
}

// applyYAML round-trips through an ordered map so a rewrite does not reshuffle
// a file the user reads. Comments are the one casualty; the alternative is a
// line editor that mangles the first block scalar it meets.
func applyYAML(src []byte, sets []field, deletes []string) ([]byte, error) {
	var doc yaml.MapSlice
	if len(bytes.TrimSpace(src)) > 0 {
		if err := yaml.UnmarshalWithOptions(src, &doc, yaml.UseOrderedMap()); err != nil {
			return nil, err
		}
	}

	for _, f := range sets {
		replaced := false
		for i := range doc {
			if fmt.Sprint(doc[i].Key) == f.key {
				doc[i].Value = f.value
				replaced = true
				break
			}
		}
		if !replaced {
			doc = append(doc, yaml.MapItem{Key: f.key, Value: f.value})
		}
	}

	for _, name := range deletes {
		kept := doc[:0]
		for _, item := range doc {
			if fmt.Sprint(item.Key) != name {
				kept = append(kept, item)
			}
		}
		doc = kept
	}

	if len(doc) == 0 {
		return []byte{}, nil
	}
	return yaml.Marshal(doc)
}

type jsonItem struct {
	key string
	raw json.RawMessage
}

func applyJSON(src []byte, sets []field, deletes []string) ([]byte, error) {
	var items []jsonItem
	if len(bytes.TrimSpace(src)) > 0 {
		decoded, err := decodeJSON(src)
		if err != nil {
			return nil, err
		}
		items = decoded
	}

	for _, f := range sets {
		encoded, err := json.Marshal(f.value)
		if err != nil {
			return nil, err
		}
		replaced := false
		for i := range items {
			if items[i].key == f.key {
				items[i].raw = encoded
				replaced = true
				break
			}
		}
		if !replaced {
			items = append(items, jsonItem{key: f.key, raw: encoded})
		}
	}

	for _, name := range deletes {
		kept := items[:0]
		for _, item := range items {
			if item.key != name {
				kept = append(kept, item)
			}
		}
		items = kept
	}

	return encodeJSON(items)
}

// decodeJSON streams the top-level object so key order survives, which
// json.Unmarshal into a map would throw away.
func decodeJSON(src []byte) ([]jsonItem, error) {
	dec := json.NewDecoder(bytes.NewReader(src))
	open, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := open.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("the file is not a JSON object")
	}

	var items []jsonItem
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}
		name, ok := key.(string)
		if !ok {
			return nil, fmt.Errorf("the file has a non-string key")
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		items = append(items, jsonItem{key: name, raw: raw})
	}
	return items, nil
}

func encodeJSON(items []jsonItem) ([]byte, error) {
	if len(items) == 0 {
		return []byte("{}\n"), nil
	}

	var out bytes.Buffer
	out.WriteString("{\n")
	for i, item := range items {
		key, err := json.Marshal(item.key)
		if err != nil {
			return nil, err
		}
		var value bytes.Buffer
		if err := json.Indent(&value, item.raw, "  ", "  "); err != nil {
			return nil, err
		}
		fmt.Fprintf(&out, "  %s: %s", key, value.String())
		if i < len(items)-1 {
			out.WriteString(",")
		}
		out.WriteString("\n")
	}
	out.WriteString("}\n")
	return out.Bytes(), nil
}

// applyTOML edits the root table line by line. Go has no TOML in its standard
// library, and for a flat config a line editor preserves more than a parser
// would — comments and layout included — without adding a dependency.
func applyTOML(src []byte, sets []field, deletes []string) ([]byte, error) {
	lines := []string{}
	if len(src) > 0 {
		lines = strings.Split(strings.TrimSuffix(string(src), "\n"), "\n")
	}

	done := map[string]bool{}
	out := make([]string, 0, len(lines)+len(sets))
	insertAt := len(lines)
	inRoot := true

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			if inRoot {
				insertAt = len(out)
				inRoot = false
			}
			out = append(out, line)
			continue
		}
		name, _, ok := tomlPair(line)
		if !ok || !inRoot {
			out = append(out, line)
			continue
		}
		if contains(deletes, name) {
			continue
		}
		if value, found := lookup(sets, name); found {
			out = append(out, fmt.Sprintf("%s = %s", name, tomlQuote(value)))
			done[name] = true
			continue
		}
		out = append(out, line)
	}
	if inRoot {
		insertAt = len(out)
	}

	var added []string
	for _, f := range sets {
		if !done[f.key] {
			added = append(added, fmt.Sprintf("%s = %s", f.key, tomlQuote(f.value)))
		}
	}
	out = append(out[:insertAt], append(added, out[insertAt:]...)...)

	body := strings.Join(out, "\n")
	if body == "" {
		return []byte{}, nil
	}
	return []byte(body + "\n"), nil
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

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func lookup(sets []field, key string) (string, bool) {
	for _, f := range sets {
		if f.key == key {
			return f.value, true
		}
	}
	return "", false
}
