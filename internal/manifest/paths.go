package manifest

import "github.com/FacileStudio/facile/internal/store"

// SourcesPath returns the path of the first sources file that exists,
// in priority order: ~/facile.yml, ~/.config/facile/sources.yml.
func SourcesPath() string { return store.SourcesPath() }

// SourcesPaths returns the ordered list of sources file paths.
func SourcesPaths() []string { return store.SourcesPaths() }
