package credstore

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type jsonItem struct {
	key string
	raw json.RawMessage
}

// readJSON returns the value for key from a JSON object, or "" when the key is
// absent.
func readJSON(src []byte, key string) (string, error) {
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
	return "", nil
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
	items, err := setJSON(items, sets)
	if err != nil {
		return nil, err
	}
	for _, name := range deletes {
		items = deleteJSON(items, name)
	}
	return encodeJSON(items)
}

// setJSON applies each field, encoding its value and replacing an existing key
// in place rather than appending a duplicate.
func setJSON(items []jsonItem, sets []field) ([]jsonItem, error) {
	for _, f := range sets {
		raw, err := json.Marshal(f.value)
		if err != nil {
			return nil, err
		}
		replaced := false
		for i := range items {
			if items[i].key == f.key {
				items[i].raw = raw
				replaced = true
				break
			}
		}
		if !replaced {
			items = append(items, jsonItem{key: f.key, raw: raw})
		}
	}
	return items, nil
}

func deleteJSON(items []jsonItem, name string) []jsonItem {
	var kept []jsonItem
	for _, item := range items {
		if item.key != name {
			kept = append(kept, item)
		}
	}
	return kept
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
