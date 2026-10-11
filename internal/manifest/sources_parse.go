package manifest

import (
	"fmt"
	"path"
	"strings"

	"github.com/goccy/go-yaml"
)

func parseRootItem(cfg *SourceConfig, key string, val any) error {
	switch strings.ToLower(key) {
	case "single", "tools":
		tools, err := parseToolsSection(val)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", key, err)
		}
		cfg.Single = append(cfg.Single, tools...)
	case "sources":
		if doc, ok := val.(yaml.MapSlice); ok {
			hasSections := false
			for _, item := range doc {
				k := strings.ToLower(fmt.Sprint(item.Key))
				if k == "tools" || k == "single" || k == "lists" || k == "sources" {
					hasSections = true
					break
				}
			}
			if hasSections {
				for _, item := range doc {
					if err := parseRootItem(cfg, fmt.Sprint(item.Key), item.Value); err != nil {
						return err
					}
				}
				return nil
			}
			tools, err := parseToolsMap(doc)
			if err != nil {
				return fmt.Errorf("parsing sources: %w", err)
			}
			cfg.Single = append(cfg.Single, tools...)
			return nil
		}
		if slice, ok := val.([]any); ok {
			tools, err := parseToolsSlice(slice)
			if err != nil {
				return fmt.Errorf("parsing sources: %w", err)
			}
			cfg.Single = append(cfg.Single, tools...)
			return nil
		}
		return fmt.Errorf("sources must be a map or a list")
	case "lists":
		lists, err := parseListSlice(val)
		if err != nil {
			return fmt.Errorf("parsing list sources: %w", err)
		}
		cfg.Lists = append(cfg.Lists, lists...)
	default:
		tool, err := parseToolEntry(key, val)
		if err != nil {
			return fmt.Errorf("parsing tool %q: %w", key, err)
		}
		cfg.Single = append(cfg.Single, tool)
	}
	return nil
}

func parseToolsSection(val any) ([]SingleSource, error) {
	if doc, ok := val.(yaml.MapSlice); ok {
		return parseToolsMap(doc)
	}
	if slice, ok := val.([]any); ok {
		return parseToolsSlice(slice)
	}
	return nil, fmt.Errorf("tools must be a map or a list")
}

func parseToolsMap(doc yaml.MapSlice) ([]SingleSource, error) {
	var out []SingleSource
	for _, item := range doc {
		s, err := parseToolEntry(fmt.Sprint(item.Key), item.Value)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func parseToolsSlice(slice []any) ([]SingleSource, error) {
	var out []SingleSource
	for _, item := range slice {
		s, err := parseToolEntry("", item)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func parseListSlice(val any) ([]ListSource, error) {
	slice, ok := val.([]any)
	if !ok {
		return nil, fmt.Errorf("lists must be an array")
	}
	var out []ListSource
	for _, item := range slice {
		ls, err := parseListItem(item)
		if err != nil {
			return nil, err
		}
		out = append(out, ls)
	}
	return out, nil
}

func parseListItem(item any) (ListSource, error) {
	if str, ok := item.(string); ok {
		name := strings.TrimSuffix(path.Base(str), path.Ext(str))
		if name == "" || name == "." || name == "/" {
			name = str
		}
		return ListSource{Name: name, URL: str}, nil
	}
	b, err := yaml.Marshal(item)
	if err != nil {
		return ListSource{}, err
	}
	var ls ListSource
	return ls, yaml.Unmarshal(b, &ls)
}

func parseToolEntry(name string, val any) (SingleSource, error) {
	if val == nil {
		return parseToolShorthand(name, ""), nil
	}
	if str, ok := val.(string); ok {
		return parseToolShorthand(name, str), nil
	}
	b, err := yaml.Marshal(val)
	if err != nil {
		return SingleSource{}, err
	}
	var s SingleSource
	if err := yaml.Unmarshal(b, &s); err != nil {
		return SingleSource{}, err
	}
	if s.Name == "" {
		s.Name = name
	}
	return s, nil
}

func parseToolShorthand(name, repoStr string) SingleSource {
	repo, branch, hasBranch := strings.Cut(repoStr, "@")
	if !hasBranch && strings.Contains(name, "@") {
		name, branch, hasBranch = strings.Cut(name, "@")
	}
	if repo == "" && strings.Contains(name, "/") {
		repo = name
		name = path.Base(name)
	}
	if name == "" && repo != "" {
		name = path.Base(repo)
	}
	s := SingleSource{Name: name, Repo: repo}
	if hasBranch {
		s.Branch = branch
	}
	return s
}
