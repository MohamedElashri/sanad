package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

func withRepositoryRoot(opts *rootOptions, run func() error) error {
	previous, err := os.Getwd()
	if err != nil {
		return categorizedError{code: exitFileSystem, err: fmt.Errorf("determine working directory: %w", err)}
	}
	root, err := repositoryRoot(previous, opts.root)
	if err != nil {
		return categorizedError{code: exitConfig, err: err}
	}
	if root == previous {
		return run()
	}
	if err := os.Chdir(root); err != nil {
		return categorizedError{code: exitFileSystem, err: fmt.Errorf("enter repository root %q: %w", root, err)}
	}
	runErr := run()
	if err := os.Chdir(previous); err != nil {
		return categorizedError{code: exitFileSystem, err: fmt.Errorf("restore working directory %q: %w", previous, err)}
	}
	return runErr
}

func repositoryRoot(start string, explicit string) (string, error) {
	if explicit != "" {
		root, err := filepath.Abs(explicit)
		if err != nil {
			return "", fmt.Errorf("resolve --root %q: %w", explicit, err)
		}
		info, err := os.Stat(root)
		if err != nil {
			return "", fmt.Errorf("resolve --root %q: %w", explicit, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("--root %q is not a directory", explicit)
		}
		return root, nil
	}

	current, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve working directory %q: %w", start, err)
	}
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Abs(start)
		}
		current = parent
	}
}
