package repo

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveRootFromRootAndNestedDirectory(t *testing.T) {
	root := t.TempDir()
	initGit(t, root)
	nested := filepath.Join(root, "one", "two")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	resolvedRoot, err := ResolveRootFrom(root)
	if err != nil {
		t.Fatal(err)
	}
	resolvedNested, err := ResolveRootFrom(nested)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedRoot != resolvedNested {
		t.Fatalf("root mismatch: %q != %q", resolvedRoot, resolvedNested)
	}
	if filepath.Clean(resolvedRoot) != filepath.Clean(root) {
		t.Fatalf("unexpected root: got %q want %q", resolvedRoot, root)
	}
}

func TestResolveRootOutsideGit(t *testing.T) {
	_, err := ResolveRootFrom(t.TempDir())
	if !errors.Is(err, ErrNotInRepo) {
		t.Fatalf("got %v, want ErrNotInRepo", err)
	}
}

func initGit(t *testing.T, directory string) {
	t.Helper()
	command := exec.Command("git", "init", "--quiet")
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, output)
	}
}
