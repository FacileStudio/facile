package manifest

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

// LoadSources reads the first-found sources file and returns the
// parsed SourceConfig. Returns an empty config (not an error) when
// no file exists.
func LoadSources() (SourceConfig, error) {
	var cfg SourceConfig
	path := SourcesPath()
	if path == "" {
		return cfg, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("reading sources file %s: %w", path, err)
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing sources file %s: %w", path, err)
	}
	return cfg, nil
}