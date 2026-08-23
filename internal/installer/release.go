package installer

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

const (
	downloadLimit  = 256 << 20
	requestTimeout = 60 * time.Second
)

// integrityError marks a release artifact that downloaded but did not verify.
// It is fatal by design: a wrong hash means the artifact is wrong, so the caller
// must not quietly compile the same version from source instead.
type integrityError struct{ msg string }

func (e integrityError) Error() string { return e.msg }

// LatestTag resolves a repository's latest release tag by following the
// /releases/latest redirect. No GitHub API, so no rate limit and no token.
// A repository facile cannot read looks exactly like one with no release, so
// the error names both causes rather than guessing between them.
func LatestTag(repo string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Head("https://github.com/" + repo + "/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	final := resp.Request.URL.Path
	_, tag, found := strings.Cut(final, "/releases/tag/")
	if !found || tag == "" {
		return "", fmt.Errorf("cannot read a release tag for %s (%s) — check that the repository is public and has a release published", repo, resp.Status)
	}
	return tag, nil
}

func fromRelease(tool manifest.Tool, version, work string) (string, error) {
	tag := version
	if tag == "" {
		resolved, err := LatestTag(tool.Repo)
		if err != nil {
			return "", err
		}
		tag = resolved
	}
	ver := strings.TrimPrefix(tag, "v")
	archive := fmt.Sprintf("%s_%s_%s_%s.tar.gz", tool.Asset, ver, runtime.GOOS, runtime.GOARCH)
	base := fmt.Sprintf("https://github.com/%s/releases/download/%s", tool.Repo, tag)

	ui.Step("Downloading %s %s for %s/%s", tool.Bin, ver, runtime.GOOS, runtime.GOARCH)
	blob, err := get(base + "/" + archive)
	if err != nil {
		return "", err
	}
	sums, err := get(base + "/checksums.txt")
	if err != nil {
		return "", err
	}
	if err := verifyChecksum(blob, archive, string(sums)); err != nil {
		return "", err
	}
	return extract(blob, tool.Bin, work)
}

// verifyChecksum aborts on a mismatch. It never falls back to a source build:
// a wrong hash means something is wrong with the artifact, not with the network.
// Both failures are integrityError, which is the type build refuses.
func verifyChecksum(blob []byte, name, sums string) error {
	sum := sha256.Sum256(blob)
	want := hex.EncodeToString(sum[:])
	for _, line := range strings.Split(sums, "\n") {
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
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != bin {
			continue
		}
		dest := filepath.Join(work, bin)
		if err := writeFrom(reader, dest); err != nil {
			return "", err
		}
		return dest, nil
	}
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

func get(url string) ([]byte, error) {
	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %s", filepath.Base(url), resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, downloadLimit))
}
