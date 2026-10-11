package manifest

import "github.com/FacileStudio/facile/internal/store"

// SourcesPath returns the path of the first sources file that exists,
// in priority order: ~/.facile.yml, ~/.facile.yaml,
// ~/.config/facile/config.yml, ~/.config/facile/config.yaml,
// ~/.config/facile/sources.yml, ~/.config/facile/sources.yaml.
func SourcesPath() string { return store.SourcesPath() }

// SourcesPaths returns the ordered list of sources file paths.
func SourcesPaths() []string { return store.SourcesPaths() }
