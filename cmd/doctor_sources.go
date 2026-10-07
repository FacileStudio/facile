package cmd

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/store"
	"github.com/FacileStudio/facile/internal/ui"
)

func checkSources() int {
	path := manifest.SourcesPath()
	if path == "" {
		return 0
	}

	ui.Hint("Sources file %s", store.Tilde(path))

	cfg, err := manifest.LoadSources()
	if err != nil {
		ui.Warn("%s", err)
		return 1
	}

	ui.Success("Sources file healthy")

	problems := 0
	if len(cfg.Lists) > 0 {
		problems += checkListSources(cfg.Lists)
	}
	problems += checkSourceShadowing(cfg)

	if m, err := catalog(); err == nil {
		for _, e := range m.LoadErrors {
			ui.Warn("%s", e)
			problems++
		}
	}

	return problems
}

func checkListSources(lists []manifest.ListSource) int {
	problems := 0
	for _, ls := range lists {
		problems += checkListSource(ls)
	}
	return problems
}

func checkListSource(ls manifest.ListSource) int {
	_, status, err := fetchURL(ls.URL)
	if err != nil {
		ui.Warn("list source %q (%s) unreachable: %s", ls.Name, ls.URL, err)
		return 1
	}
	if status < 200 || status >= 300 {
		ui.Warn("list source %q (%s) returned %d", ls.Name, ls.URL, status)
		return 1
	}
	ui.Success("list source %q reachable", ls.Name)
	return 0
}

func checkSourceShadowing(cfg manifest.SourceConfig) int {
	base, err := catalogBase()
	if err != nil {
		return 0
	}
	catalogNames := make(map[string]bool, len(base.Tools))
	for _, t := range base.Tools {
		catalogNames[strings.ToLower(t.Name)] = true
	}
	problems := 0
	for _, s := range cfg.Single {
		if s.Name != "" && catalogNames[strings.ToLower(s.Name)] {
			ui.Warn("user source %q shadows the catalog tool of the same name", s.Name)
			problems++
		}
	}
	return problems
}

func catalogBase() (*manifest.Manifest, error) {
	return manifest.LoadBase(store.CatalogPath())
}

func fetchURL(url string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "text/plain, text/html, application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}