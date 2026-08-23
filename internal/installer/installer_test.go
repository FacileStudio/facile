package installer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FacileStudio/facile/internal/manifest"
)

func TestVerifyChecksumRejectsATamperedArchive(t *testing.T) {
	blob := []byte("the real artifact")
	sum := sha256.Sum256(blob)
	sums := hex.EncodeToString(sum[:]) + "  tool_1.0.0_darwin_arm64.tar.gz\n"

	if err := verifyChecksum(blob, "tool_1.0.0_darwin_arm64.tar.gz", sums); err != nil {
		t.Fatalf("a matching checksum must pass: %v", err)
	}

	var integrity integrityError
	if err := verifyChecksum([]byte("swapped"), "tool_1.0.0_darwin_arm64.tar.gz", sums); !errors.As(err, &integrity) {
		t.Fatalf("a mismatched checksum must abort as an integrity failure, got %v", err)
	}
	if err := verifyChecksum(blob, "absent.tar.gz", sums); !errors.As(err, &integrity) {
		t.Fatalf("an unlisted archive must abort as an integrity failure, got %v", err)
	}
	if errors.As(fmt.Errorf("tool_1.0.0_darwin_arm64.tar.gz returned 404 Not Found"), &integrity) {
		t.Fatal("a transport failure must stay a source-build fallback")
	}
}

// The rule is CLAUDE.md's: a checksum mismatch aborts. It is enforced in build,
// not in verifyChecksum. Detection was always correct; the decision above it threw
// the result away, and the test above passed throughout.
func TestBuildRefusesASourceFallbackAfterAnIntegrityFailure(t *testing.T) {
	stubRelease(t, func(manifest.Tool, string, string) (string, error) {
		return "", integrityError{"checksum mismatch for tool_1.0.0_darwin_arm64.tar.gz — re-cut the release"}
	})

	_, err := build(fakeTool(), Options{}, t.TempDir())

	var integrity integrityError
	if !errors.As(err, &integrity) {
		t.Fatalf("an integrity failure must abort the install, got %v", err)
	}
}

func TestBuildStillFallsBackWhenAReleaseIsMerelyUnreachable(t *testing.T) {
	stubRelease(t, func(manifest.Tool, string, string) (string, error) {
		return "", fmt.Errorf("tool_1.0.0_darwin_arm64.tar.gz returned 404 Not Found")
	})

	_, err := build(fakeTool(), Options{}, t.TempDir())

	if err == nil || !strings.Contains(err.Error(), "unknown build backend") {
		t.Fatalf("a transport failure must reach the source build, got %v", err)
	}
}

func stubRelease(t *testing.T, fn func(manifest.Tool, string, string) (string, error)) {
	t.Helper()
	original := fromReleaseFn
	t.Cleanup(func() { fromReleaseFn = original })
	fromReleaseFn = fn
}

// The build backend is deliberately unknown: fromSource rejects it before it can
// reach git, so a fallback is visible in the error without touching the network.
func fakeTool() manifest.Tool {
	return manifest.Tool{Bin: "tool", Asset: "tool", Repo: "FacileStudio/tool", Branch: "main", Build: "unknown"}
}

func TestExtractFindsTheBinaryAndIgnoresTheRest(t *testing.T) {
	archive := tarGz(t, map[string]string{
		"README.md": "not the binary",
		"opus":      "#!/bin/sh\necho opus 1.2.3\n",
	})
	work := t.TempDir()

	path, err := extract(archive, "opus", work)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(body), "opus 1.2.3") {
		t.Fatalf("extracted the wrong member: %q, %v", body, err)
	}
	if _, err := extract(archive, "sablier", work); err == nil {
		t.Fatal("an archive without the expected binary must be an error")
	}
}

// atomicInstall exists so that updating a tool while it is running cannot
// corrupt the running image. Replacing the destination in place is the bug.
func TestAtomicInstallLeavesNoStagedFileBehind(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "opus")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := atomicInstall(src, dest); err != nil {
		t.Fatalf("atomicInstall: %v", err)
	}
	body, _ := os.ReadFile(dest)
	if string(body) != "new" {
		t.Fatalf("destination not replaced: %q", body)
	}
	info, err := os.Stat(dest)
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("installed binary must be executable, got %v", info.Mode().Perm())
	}
	staged, _ := filepath.Glob(filepath.Join(dir, ".opus.new.*"))
	if len(staged) != 0 {
		t.Fatalf("staged file left behind: %v", staged)
	}
}

func tarGz(t *testing.T, members map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range members {
		header := &tar.Header{
			Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}
