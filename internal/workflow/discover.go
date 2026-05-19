package workflow

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func DiscoverWorkflowFiles(paths []string) ([]string, error) {
	seen := make(map[string]struct{})
	var files []string

	for _, path := range paths {
		path = filepath.Clean(path)
		matches, err := discoverWorkflowFiles(path)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			if _, ok := seen[match]; ok {
				continue
			}
			seen[match] = struct{}{}
			files = append(files, match)
		}
	}

	sort.Strings(files)
	return files, nil
}

func discoverWorkflowFiles(path string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			if current == path && isNotExist(err) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if isWorkflowYAML(current) {
				return fmt.Errorf("workflow path %q must not be a symlink", current)
			}
			return nil
		}
		if isWorkflowYAML(current) {
			files = append(files, normalizeWorkflowPath(current))
		}
		return nil
	})
	if err != nil && isNotExist(err) {
		return nil, nil
	}

	return files, err
}

func isWorkflowYAML(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yml" || ext == ".yaml"
}

func normalizeWorkflowPath(path string) string {
	return strings.ReplaceAll(filepath.ToSlash(path), "\\", "/")
}

func isNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
