package manifest

import (
	"bytes"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

// LoadSources reads the first-found sources file and returns the
// parsed SourceConfig. Returns an empty config (not an error) when
// no file exists.
func LoadSources() (SourceConfig, error) {
	path := SourcesPath()
	if path == "" {
		return SourceConfig{}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return SourceConfig{}, fmt.Errorf("reading sources file %s: %w", path, err)
	}
	cfg, err := ParseSources(raw)
	if err != nil {
		return SourceConfig{}, fmt.Errorf("parsing sources file %s: %w", path, err)
	}
	return cfg, nil
}

// ParseSources parses YAML content into a SourceConfig, supporting
// root-level tool maps, explicit tools blocks, and legacy formats.
func ParseSources(raw []byte) (SourceConfig, error) {
	var cfg SourceConfig
	if len(bytes.TrimSpace(raw)) == 0 {
		return cfg, nil
	}
	var doc yaml.MapSlice
	if err := yaml.UnmarshalWithOptions(raw, &doc, yaml.UseOrderedMap()); err != nil {
		var seq []any
		if seqErr := yaml.Unmarshal(raw, &seq); seqErr == nil {
			tools, err := parseToolsSection(seq)
			cfg.Single = tools
			return cfg, err
		}
		return cfg, fmt.Errorf("parsing sources: %w", err)
	}
	for _, item := range doc {
		if err := parseRootItem(&cfg, fmt.Sprint(item.Key), item.Value); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}
