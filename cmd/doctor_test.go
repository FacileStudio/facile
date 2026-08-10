package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// A bin dir reached through a symlink is still the bin dir. Comparing a
// resolved path against an unresolved one reported every tool under macOS's
// /tmp as its own impostor.
func TestRealPathSeesThroughASymlinkedDirectory(t *testing.T) {
	root := t.TempDir()
	actual := filepath.Join(root, "actual")
	if err := os.MkdirAll(actual, 0o755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(actual, "opus")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(actual, link); err != nil {
		t.Skip("this filesystem does not support symlinks")
	}

	if realPath(filepath.Join(link, "opus")) != realPath(binary) {
		t.Fatal("the same binary reached two ways must compare equal")
	}
}
