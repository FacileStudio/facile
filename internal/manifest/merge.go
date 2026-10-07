package manifest

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Merge combines a base manifest with any number of override layers.
// Each layer's tools shadow the base and previous layers by name
// (case-insensitive, matching Tool.Get semantics). Tools from later
// layers are appended in order after earlier tools that are not
// shadowed. This is the same semantics as FACILE_CATALOG: a
// user source named "sablier" overrides the suite catalog entry.
//
// The base is never mutated: Merge builds a fresh manifest.
func Merge(base *Manifest, layers ...*Manifest) *Manifest {
	if base == nil {
		return nil
	}
	out := &Manifest{
		Version: base.Version,
		MCP:     append([]string{}, base.MCP...),
	}
	index := make(map[string]int, len(base.Tools))
	for _, t := range base.Tools {
		key := strings.ToLower(t.Name)
		index[key] = len(out.Tools)
		out.Tools = append(out.Tools, t)
	}
	for _, layer := range layers {
		if layer == nil {
			continue
		}
		for _, t := range layer.Tools {
			key := strings.ToLower(t.Name)
			if i, ok := index[key]; ok {
				out.Tools[i] = t
				continue
			}
			index[key] = len(out.Tools)
			out.Tools = append(out.Tools, t)
		}
	}
	return out
}

// mergeCatalogs overlays user sources on top of the base manifest.
// User sources are loaded from ~/facile.yml or ~/.config/facile/sources.yml
// (first found wins), including repos discovered via list sources. A broken
// or missing sources file is never fatal — the base is returned unchanged.
// Non-fatal merge errors (unreachable list URLs, unparseable repo configs,
// broken SingleSource entries) are collected in LoadErrors.
func mergeCatalogs(base *Manifest) *Manifest {
	cfg, err := LoadSources()
	if err != nil {
		return base
	}

	loadErrors, layers := collectLayers(cfg, base.Version)
	if len(layers) == 0 {
		if len(loadErrors) > 0 {
			out := *base
			out.LoadErrors = loadErrors
			return &out
		}
		return base
	}
	merged := Merge(base, layers...)
	merged.LoadErrors = loadErrors
	return merged
}

func collectLayers(cfg SourceConfig, version int) ([]string, []*Manifest) {
	var loadErrors []string
	var layers []*Manifest

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if len(cfg.Single) > 0 {
		tools, errs := processSingleSources(ctx, cfg.Single)
		loadErrors = append(loadErrors, errs...)
		layers = append(layers, &Manifest{Version: version, Tools: tools})
	}

	if len(cfg.Lists) > 0 {
		discovered, errs := DiscoverToolsFromListSources(ctx, cfg)
		for _, e := range errs {
			loadErrors = append(loadErrors, e.Error())
		}
		if len(discovered) > 0 {
			layers = append(layers, &Manifest{Version: version, Tools: discovered})
		}
	}

	return loadErrors, layers
}

func processSingleSources(ctx context.Context, singles []SingleSource) ([]Tool, []string) {
	var tools []Tool
	var errors []string

	for _, s := range singles {
		if s.Repo != "" && s.Name == "" {
			tool, err := discoverTool(ctx, s.Repo)
			if err != nil {
				errors = append(errors, fmt.Sprintf("discovering %s: %s", s.Repo, err))
				continue
			}
			if tool == nil {
				errors = append(errors, fmt.Sprintf("no facile.toml or [facile] block found in %s", s.Repo))
				continue
			}
			tools = append(tools, *tool)
			continue
		}
		tool, err := s.ToTool()
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		tools = append(tools, tool)
	}
	return tools, errors
}
