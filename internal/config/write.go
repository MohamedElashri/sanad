package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	dottedUpdatesBranchesLinePattern = regexp.MustCompile(`(?m)^(\s*updates\.branches\s*=\s*)("[^"]*"|'[^']*'|[^\s#]+)(\s*(?:#.*)?)$`)
	branchesLinePattern              = regexp.MustCompile(`(?m)^(\s*branches\s*=\s*)("[^"]*"|'[^']*'|[^\s#]+)(\s*(?:#.*)?)$`)
	updatesHeaderPattern             = regexp.MustCompile(`(?m)^\s*\[updates\]\s*(?:#.*)?$`)
	tableHeaderPattern               = regexp.MustCompile(`(?m)^\s*\[[^\]]+\]\s*(?:#.*)?$`)
)

func PersistBranchTracking(path string) error {
	if path == "" {
		path = DefaultPath
	}
	return writeUpdatesBranches(path, "track")
}

func writeUpdatesBranches(path string, value string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && path == DefaultPath {
			data = nil
		} else {
			return fmt.Errorf("write config %q: %w", path, err)
		}
	}

	next, err := setUpdatesBranches(string(data), value)
	if err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	if _, _, err := decodeConfigData([]byte(next)); err != nil {
		return fmt.Errorf("write config %q: generated invalid TOML: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	perm := os.FileMode(0o600)
	if info, statErr := os.Stat(path); statErr == nil {
		perm = info.Mode().Perm()
	}
	if err := os.WriteFile(path, []byte(next), perm); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	return nil
}

func setUpdatesBranches(data string, value string) (string, error) {
	line := fmt.Sprintf(`branches = "%s"`, value)
	if data == "" {
		return "[updates]\n" + line + "\n", nil
	}

	if dottedUpdatesBranchesLinePattern.MatchString(data) {
		updated := dottedUpdatesBranchesLinePattern.ReplaceAllString(data, `${1}"`+value+`"${3}`)
		return ensureTrailingNewline(updated), nil
	}

	header := updatesHeaderPattern.FindStringIndex(data)
	if header == nil {
		return appendUpdatesSection(data, line), nil
	}

	insertAt := nextTableHeaderOffset(data, header[1])
	section := data[header[1]:insertAt]
	if branchesLinePattern.MatchString(section) {
		updatedSection := branchesLinePattern.ReplaceAllString(section, `${1}"`+value+`"${3}`)
		return ensureTrailingNewline(data[:header[1]] + updatedSection + data[insertAt:]), nil
	}

	prefix := strings.TrimRight(data[:insertAt], "\r\n")
	suffix := data[insertAt:]
	separator := "\n"
	if strings.Contains(data[:insertAt], "\r\n") {
		separator = "\r\n"
	}
	return prefix + separator + line + separator + suffix, nil
}

func appendUpdatesSection(data string, line string) string {
	separator := "\n"
	if strings.Contains(data, "\r\n") {
		separator = "\r\n"
	}
	trimmed := strings.TrimRight(data, "\r\n")
	if trimmed == "" {
		return "[updates]" + separator + line + separator
	}
	return trimmed + separator + separator + "[updates]" + separator + line + separator
}

func nextTableHeaderOffset(data string, start int) int {
	loc := tableHeaderPattern.FindStringIndex(data[start:])
	if loc == nil {
		return len(data)
	}
	return start + loc[0]
}

func ensureTrailingNewline(data string) string {
	if strings.HasSuffix(data, "\n") || strings.HasSuffix(data, "\r\n") {
		return data
	}
	return data + "\n"
}
