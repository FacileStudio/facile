package manifest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/FacileStudio/facile/internal/store"
)

func deduplicate(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// ParseListSource extracts repo identifiers from a list source body in either
// format: one per line (owner/repo, skipping comments and empty lines) or a
// PEP 503 simple index (HTML with <a href="/owner/repo/"> links).
func ParseListSource(body string) []string {
	var repos []string
	linkRe := regexp.MustCompile(`<a href="/([^/]+/[^/]+)/?">`)
	for _, m := range linkRe.FindAllStringSubmatch(body, -1) {
		if len(m) == 2 {
			repos = append(repos, m[1])
		}
	}
	if len(repos) > 0 {
		return deduplicate(repos)
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Count(line, "/") == 1 {
			repos = append(repos, line)
		}
	}
	return deduplicate(repos)
}

func listSourceCachePath(url string) string {
	hash := sha256.Sum256([]byte(url))
	return filepath.Join(store.CacheDir(), "lists", hex.EncodeToString(hash[:]))
}

func fetchListSource(ctx context.Context, url string) ([]string, error) {
	cachePath := listSourceCachePath(url)
	if repos := readCachedList(cachePath); repos != nil {
		return repos, nil
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/plain, text/html, application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list source %s returned %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	repos := ParseListSource(string(body))
	writeListCache(cachePath, repos)
	return repos, nil
}

func writeListCache(cachePath string, repos []string) {
	content := strings.Join(repos, "\n")
	if len(repos) == 0 {
		content = "# empty"
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err == nil {
		_ = os.WriteFile(cachePath, []byte(content), 0o644)
	}
}

func readCachedList(cachePath string) []string {
	info, err := os.Stat(cachePath)
	if err != nil || time.Since(info.ModTime()) >= 24*time.Hour {
		return nil
	}
	raw, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}
	content := strings.TrimSpace(string(raw))
	if content == "# empty" {
		return []string{}
	}
	if content == "" {
		return nil
	}
	var repos []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			repos = append(repos, line)
		}
	}
	return repos
}

// DiscoverToolsFromListSources fetches and parses facile.toml or mise.toml/[facile]
// for each repo returned by the configured list sources.
func DiscoverToolsFromListSources(ctx context.Context, cfg SourceConfig) ([]Tool, []error) {
	var tools []Tool
	var errs []error
	for _, ls := range cfg.Lists {
		t, e := discoverFromList(ctx, ls)
		tools = append(tools, t...)
		errs = append(errs, e...)
	}
	return tools, errs
}

func discoverFromList(ctx context.Context, ls ListSource) ([]Tool, []error) {
	var tools []Tool
	var errs []error
	repos, err := fetchListSource(ctx, ls.URL)
	if err != nil {
		errs = append(errs, fmt.Errorf("list source %q (%s): %w", ls.Name, ls.URL, err))
		return tools, errs
	}
	for _, repo := range repos {
		tool, err := discoverTool(ctx, repo)
		if err != nil {
			errs = append(errs, fmt.Errorf("discovering %s from list %q: %w", repo, ls.Name, err))
			continue
		}
		if tool != nil {
			tools = append(tools, *tool)
		}
	}
	return tools, errs
}

func discoverTool(ctx context.Context, repo string) (*Tool, error) {
	tool, err := probeRepoFile(ctx, repo, "facile.toml")
	if tool != nil {
		return tool, nil
	}
	tool2, err2 := probeRepoFile(ctx, repo, "mise.toml")
	if tool2 != nil {
		return tool2, nil
	}
	if err2 != nil {
		return nil, err2
	}
	return nil, err
}

func probeRepoFile(ctx context.Context, repo, file string) (*Tool, error) {
	var lastNon404 error
	for _, branch := range []string{"main", "master"} {
		tool, err := probeBranch(ctx, repo, file, branch)
		if err != nil {
			if !isNotFound(err) {
				lastNon404 = err
			}
			continue
		}
		if tool != nil {
			return tool, nil
		}
	}
	return nil, lastNon404
}

func probeBranch(ctx context.Context, repo, file, branch string) (*Tool, error) {
	parseFn := parseMiseTomlFacade
	probeFn := miseTomlFacadeProbe
	if file == "facile.toml" {
		parseFn = parseFacileToml
		probeFn = facileTomlProbe
	}
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", repo, branch, file)
	raw, err := fetchWithContext(ctx, url)
	if err != nil {
		return nil, err
	}
	if !probeFn(raw) {
		return nil, nil
	}
	ft, err := parseFn(raw)
	if err != nil {
		if errors.Is(err, ErrNoFacileToml) {
			return nil, nil
		}
		return nil, fmt.Errorf("parsing %s/%s@%s: %w", repo, file, branch, err)
	}
	tool, err := ft.ToTool()
	if err != nil {
		return nil, err
	}
	return &tool, nil
}

type httpStatusError struct {
	code int
	msg  string
}

func (e *httpStatusError) Error() string { return e.msg }

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var s *httpStatusError
	return errors.As(err, &s) && s.code == 404
}

func fetchWithContext(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &httpStatusError{code: resp.StatusCode, msg: fmt.Sprintf("status %s", resp.Status)}
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}