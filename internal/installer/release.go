package installer

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

const (
	downloadLimit    = 256 << 20
	requestTimeout   = 60 * time.Second
	downloadAttempts = 3
	latestTimeout    = 15 * time.Second
)

var (
	githubBase = "https://github.com"
	apiBase    = "https://api.github.com"
	retryDelay = time.Second
)

// integrityError marks a release artifact that downloaded but did not verify.
// It is fatal by design: a wrong hash means the artifact is wrong, so the caller
// must not quietly compile the same version from source instead.
type integrityError struct{ msg string }

func (e integrityError) Error() string { return e.msg }

// LatestTag resolves a repository's latest release tag by following the
// /releases/latest redirect, with no token and no API rate limit. GitHub's web
// front sometimes stalls or answers 5xx, so the redirect is tried twice and the
// API answers last. A repository facile cannot read looks exactly like one with
// no release, so the error names both causes rather than guessing between them.
func LatestTag(repo string) (string, error) {
	var err error
	for range 2 {
		var tag string
		if tag, err = redirectTag(repo); err == nil {
			return tag, nil
		}
		if _, missing := err.(noReleaseError); missing {
			return "", err
		}
	}
	tag, apiErr := apiLatestTag(repo)
	if apiErr != nil {
		return "", fmt.Errorf("%w (API fallback: %v)", err, apiErr)
	}
	return tag, nil
}

type noReleaseError struct{ msg string }

func (e noReleaseError) Error() string { return e.msg }

func redirectTag(repo string) (string, error) {
	client := &http.Client{Timeout: latestTimeout}
	resp, err := client.Head(githubBase + "/" + repo + "/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return "", statusError{name: repo + " releases/latest", code: resp.StatusCode, status: resp.Status}
	}
	_, tag, found := strings.Cut(resp.Request.URL.Path, "/releases/tag/")
	if !found || tag == "" {
		return "", noReleaseError{fmt.Sprintf("cannot read a release tag for %s (%s) — check that the repository is public and has a release published", repo, resp.Status)}
	}
	return tag, nil
}

func apiLatestTag(repo string) (string, error) {
	body, err := get(fmt.Sprintf("%s/repos/%s/releases/latest", apiBase, repo),
		apiHeaders("application/vnd.github+json"))
	if err != nil {
		return "", err
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &release); err != nil || release.TagName == "" {
		return "", fmt.Errorf("cannot read the latest release of %s", repo)
	}
	return release.TagName, nil
}

func fromRelease(tool manifest.Tool, version, work string) ([]string, error) {
	tag := version
	if tag == "" {
		resolved, err := LatestTag(tool.Repo)
		if err != nil {
			return nil, err
		}
		tag = resolved
	}
	ver := strings.TrimPrefix(tag, "v")
	archive := fmt.Sprintf("%s_%s_%s_%s.tar.gz", tool.Asset, ver, runtime.GOOS, runtime.GOARCH)

	ui.Step("Downloading %s %s for %s/%s", tool.Bin, ver, runtime.GOOS, runtime.GOARCH)
	blob, err := releaseAsset(tool.Repo, tag, archive)
	if err != nil {
		return nil, err
	}
	sums, err := releaseAsset(tool.Repo, tag, "checksums.txt")
	if err != nil {
		return nil, err
	}
	if err := verifyChecksum(blob, archive, string(sums)); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(tool.BinList()))
	for _, bin := range tool.BinList() {
		path, err := extract(blob, bin, work)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// verifyChecksum aborts on a mismatch. It never falls back to a source build:
// a wrong hash means something is wrong with the artifact, not with the network.
// Both failures are integrityError, which is the type build refuses.
func verifyChecksum(blob []byte, name, sums string) error {
	sum := sha256.Sum256(blob)
	want := hex.EncodeToString(sum[:])
	for line := range strings.SplitSeq(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			if fields[0] == want {
				return nil
			}
			return integrityError{fmt.Sprintf("checksum mismatch for %s — re-cut the release, this artifact is not the one that was published", name)}
		}
	}
	return integrityError{fmt.Sprintf("%s is not listed in checksums.txt — re-cut the release", name)}
}

func extract(blob []byte, bin, work string) (string, error) {
	gz, err := gzip.NewReader(strings.NewReader(string(blob)))
	if err != nil {
		return "", fmt.Errorf("the downloaded archive is not gzip")
	}
	defer gz.Close()

	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return "", fmt.Errorf("the archive contains no %s binary", bin)
		}
		if err != nil {
			return "", fmt.Errorf("cannot read the archive: %w", err)
		}
		if !isBinaryEntry(header, bin) {
			continue
		}
		dest := filepath.Join(work, bin)
		if err := writeFrom(reader, dest); err != nil {
			return "", err
		}
		return dest, nil
	}
}

// isBinaryEntry is true for a plain file whose basename is the tool's binary.
func isBinaryEntry(header *tar.Header, bin string) bool {
	return header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == bin
}

func writeFrom(r io.Reader, dest string) error {
	file, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, io.LimitReader(r, downloadLimit))
	return err
}

// releaseAsset downloads one file of a release. GitHub's public download URL
// sometimes answers 5xx for an asset that exists, while the API serves the same
// bytes, so a server error is retried and then fetched through the API. The
// bytes are verified against checksums.txt either way.
func releaseAsset(repo, tag, name string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/releases/download/%s/%s", githubBase, repo, tag, name)
	var err error
	for attempt := range downloadAttempts {
		var blob []byte
		if blob, err = get(url, nil); err == nil {
			return blob, nil
		}
		var status statusError
		if !errors.As(err, &status) || status.code < 500 {
			return nil, err
		}
		time.Sleep(retryDelay * time.Duration(attempt+1))
	}
	blob, apiErr := apiAsset(repo, tag, name)
	if apiErr != nil {
		return nil, fmt.Errorf("%w (API fallback: %v)", err, apiErr)
	}
	return blob, nil
}

func apiAsset(repo, tag, name string) ([]byte, error) {
	meta, err := get(fmt.Sprintf("%s/repos/%s/releases/tags/%s", apiBase, repo, tag),
		apiHeaders("application/vnd.github+json"))
	if err != nil {
		return nil, err
	}
	var release struct {
		Assets []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(meta, &release); err != nil {
		return nil, fmt.Errorf("cannot read the release of %s: %w", repo, err)
	}
	for _, asset := range release.Assets {
		if asset.Name == name {
			return get(fmt.Sprintf("%s/repos/%s/releases/assets/%d", apiBase, repo, asset.ID),
				apiHeaders("application/octet-stream"))
		}
	}
	return nil, fmt.Errorf("%s is not an asset of %s %s", name, repo, tag)
}

// apiHeaders authenticates API calls when a GitHub token is at hand. GitHub
// throttles and times out anonymous clients first, so a signed-in gh keeps the
// fallback working. The token goes to api.github.com only, never to a download
// host.
func apiHeaders(accept string) map[string]string {
	headers := map[string]string{"Accept": accept}
	if token := githubToken(); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return headers
}

var githubToken = sync.OnceValue(func() string {
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if token := strings.TrimSpace(os.Getenv(name)); token != "" {
			return token
		}
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return ""
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
})

type statusError struct {
	name   string
	code   int
	status string
}

func (e statusError) Error() string { return fmt.Sprintf("%s returned %s", e.name, e.status) }

func get(url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, statusError{name: filepath.Base(url), code: resp.StatusCode, status: resp.Status}
	}
	return io.ReadAll(io.LimitReader(resp.Body, downloadLimit))
}
