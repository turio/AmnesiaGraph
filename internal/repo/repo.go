package repo

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNotInRepo is returned when the current directory is not inside a Git
// repository or Git cannot resolve its root.
var ErrNotInRepo = errors.New("NOT_IN_REPO")

// ResolveRoot resolves the Git root for the process's current directory.
func ResolveRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", ErrNotInRepo
	}
	return ResolveRootFrom(cwd)
}

// ResolveRootFrom resolves the Git root for cwd. Keeping cwd explicit makes
// the repo gate easy to exercise without changing process-global state.
func ResolveRootFrom(cwd string) (string, error) {
	if cwd == "" {
		return "", ErrNotInRepo
	}

	command := exec.Command("git", "rev-parse", "--show-toplevel")
	command.Dir = cwd
	output, err := command.Output()
	if err != nil {
		return "", ErrNotInRepo
	}

	root := strings.TrimSpace(string(output))
	if root == "" {
		return "", ErrNotInRepo
	}
	root, err = filepath.Abs(filepath.FromSlash(root))
	if err != nil {
		return "", ErrNotInRepo
	}
	root = filepath.Clean(root)
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = filepath.Clean(resolved)
	}
	return root, nil
}
