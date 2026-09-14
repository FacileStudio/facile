package credstore

import (
	"bytes"
	"fmt"

	"github.com/goccy/go-yaml"
)

// readYAML returns the value for key from a YAML document, or "" when the key
// is absent.
func readYAML(src []byte, key string) (string, error) {
	var doc yaml.MapSlice
	if err := yaml.UnmarshalWithOptions(src, &doc, yaml.UseOrderedMap()); err != nil {
		return "", err
	}
	for _, item := range doc {
		if fmt.Sprint(item.Key) == key {
			return fmt.Sprint(item.Value), nil
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
		doc = setYAML(doc, f)
	}
	for _, name := range deletes {
		doc = deleteYAML(doc, name)
	}
	if len(doc) == 0 {
		return []byte{}, nil
	}
	return yaml.Marshal(doc)
}

// setYAML sets one field, replacing an existing key in place rather than
// appending a duplicate.
func setYAML(doc yaml.MapSlice, f field) yaml.MapSlice {
	for i := range doc {
		if fmt.Sprint(doc[i].Key) == f.key {
			doc[i].Value = f.value
			return doc
		}
	}
	return append(doc, yaml.MapItem{Key: f.key, Value: f.value})
}

func deleteYAML(doc yaml.MapSlice, name string) yaml.MapSlice {
	var kept yaml.MapSlice
	for _, item := range doc {
		if fmt.Sprint(item.Key) != name {
			kept = append(kept, item)
		}
	}
	return kept
}
