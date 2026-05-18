package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && path == DefaultPath {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	}
	if info.IsDir() {
		return Config{}, fmt.Errorf("load config %q: path is a directory", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	}

	if organization, ok, err := parseOrganization(string(data), cfg.Organization); err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	} else if ok {
		for _, policyFile := range organization.PolicyFiles {
			resolvedPath := policyFile
			if !filepath.IsAbs(resolvedPath) {
				resolvedPath = filepath.Join(filepath.Dir(path), resolvedPath)
			}
			policyData, err := os.ReadFile(resolvedPath)
			if err != nil {
				return Config{}, fmt.Errorf("load organization policy %q: %w", policyFile, err)
			}
			cfg, err = applyConfigData(cfg, resolvedPath, string(policyData))
			if err != nil {
				return Config{}, err
			}
		}
	}

	cfg, err = applyConfigData(cfg, path, string(data))
	if err != nil {
		return Config{}, err
	}

	cfg.Source = path
	return cfg, nil
}

func applyConfigData(cfg Config, path string, data string) (Config, error) {
	if workflowPaths, ok, err := parseWorkflowPaths(data); err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	} else if ok {
		cfg.WorkflowPaths = workflowPaths
	}

	if cooldown, ok, err := parseCooldown(data); err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	} else if ok {
		cfg.Cooldown = cooldown
	}

	if updates, ok, err := parseUpdates(data, cfg.Updates); err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	} else if ok {
		cfg.Updates = updates
	}

	if ignore, ok, err := parseIgnore(data, cfg.Ignore); err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	} else if ok {
		cfg.Ignore = ignore
	}

	if github, ok, err := parseGitHub(data, cfg.GitHub); err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	} else if ok {
		cfg.GitHub = github
	}

	if organization, ok, err := parseOrganization(data, cfg.Organization); err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	} else if ok {
		cfg.Organization = organization
	}

	return cfg, nil
}

func ParseDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("duration must not be empty")
	}

	var duration time.Duration
	var err error
	if strings.HasSuffix(value, "d") {
		days := strings.TrimSuffix(value, "d")
		if days == "" || strings.HasPrefix(days, "+") || strings.HasPrefix(days, "-") {
			return 0, fmt.Errorf("invalid duration %q", value)
		}
		n, parseErr := strconv.ParseInt(days, 10, 64)
		if parseErr != nil {
			return 0, fmt.Errorf("invalid duration %q", value)
		}
		duration = time.Duration(n) * 24 * time.Hour
	} else {
		duration, err = time.ParseDuration(value)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", value)
		}
	}

	if duration < 0 {
		return 0, fmt.Errorf("duration must not be negative")
	}
	return duration, nil
}

func parseCooldown(data string) (time.Duration, bool, error) {
	lines := strings.Split(data, "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(stripInlineComment(rawLine))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			return 0, false, nil
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "cooldown" {
			continue
		}

		text, err := parseStringValue(strings.TrimSpace(value), "cooldown")
		if err != nil {
			return 0, true, err
		}
		duration, err := ParseDuration(text)
		if err != nil {
			return 0, true, fmt.Errorf("cooldown: %w", err)
		}
		return duration, true, nil
	}

	return 0, false, nil
}

func parseWorkflowPaths(data string) ([]string, bool, error) {
	lines := strings.Split(data, "\n")
	for i := 0; i < len(lines); i++ {
		line := stripInlineComment(lines[i])
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "workflow_paths" {
			continue
		}

		body := strings.TrimSpace(value)
		for !strings.Contains(body, "]") && i+1 < len(lines) {
			i++
			body += "\n" + stripInlineComment(lines[i])
		}

		paths, err := parseStringArray(body, "workflow_paths")
		return paths, true, err
	}

	return nil, false, nil
}

func parseUpdates(data string, defaults UpdatesConfig) (UpdatesConfig, bool, error) {
	section, ok := sectionBody(data, "updates")
	if !ok {
		return UpdatesConfig{}, false, nil
	}

	updates := defaults
	if value, ok, err := stringSectionValue(section, "tags"); err != nil {
		return UpdatesConfig{}, true, err
	} else if ok {
		updates.Tags = value
	}
	if value, ok, err := stringSectionValue(section, "branches"); err != nil {
		return UpdatesConfig{}, true, err
	} else if ok {
		updates.Branches = value
	}
	if value, ok, err := stringSectionValue(section, "unpinned"); err != nil {
		return UpdatesConfig{}, true, err
	} else if ok {
		updates.Unpinned = value
	}
	if value, ok, err := boolSectionValue(section, "reusable_workflows"); err != nil {
		return UpdatesConfig{}, true, err
	} else if ok {
		updates.ReusableWorkflows = value
	}

	return updates, true, nil
}

func parseIgnore(data string, defaults IgnoreConfig) (IgnoreConfig, bool, error) {
	section, ok := sectionBody(data, "ignore")
	if !ok {
		return IgnoreConfig{}, false, nil
	}

	ignore := defaults
	if value, ok, err := arraySectionValue(section, "actions"); err != nil {
		return IgnoreConfig{}, true, err
	} else if ok {
		ignore.Actions = value
	}
	if value, ok, err := arraySectionValue(section, "files"); err != nil {
		return IgnoreConfig{}, true, err
	} else if ok {
		ignore.Files = value
	}
	return ignore, true, nil
}

func sectionBody(data string, section string) (string, bool) {
	lines := strings.Split(data, "\n")
	var body []string
	inSection := false

	for _, rawLine := range lines {
		line := strings.TrimSpace(stripInlineComment(rawLine))
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			if inSection && name != section {
				break
			}
			inSection = name == section
			continue
		}
		if inSection {
			body = append(body, rawLine)
		}
	}

	if !inSection && len(body) == 0 {
		return "", false
	}
	return strings.Join(body, "\n"), true
}

func stringSectionValue(section string, key string) (string, bool, error) {
	for _, rawLine := range strings.Split(section, "\n") {
		line := strings.TrimSpace(stripInlineComment(rawLine))
		if line == "" {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(name) != key {
			continue
		}
		parsed, err := parseStringValue(strings.TrimSpace(value), key)
		return parsed, true, err
	}
	return "", false, nil
}

func boolSectionValue(section string, key string) (bool, bool, error) {
	for _, rawLine := range strings.Split(section, "\n") {
		line := strings.TrimSpace(stripInlineComment(rawLine))
		if line == "" {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(name) != key {
			continue
		}
		switch strings.TrimSpace(value) {
		case "true":
			return true, true, nil
		case "false":
			return false, true, nil
		default:
			return false, true, fmt.Errorf("%s must be a TOML boolean", key)
		}
	}
	return false, false, nil
}

func arraySectionValue(section string, key string) ([]string, bool, error) {
	lines := strings.Split(section, "\n")
	for i := 0; i < len(lines); i++ {
		line := stripInlineComment(lines[i])
		name, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(name) != key {
			continue
		}

		body := strings.TrimSpace(value)
		for !strings.Contains(body, "]") && i+1 < len(lines) {
			i++
			body += "\n" + stripInlineComment(lines[i])
		}

		values, err := parseStringArray(body, key)
		return values, true, err
	}

	return nil, false, nil
}

func parseGitHub(data string, defaults GitHubConfig) (GitHubConfig, bool, error) {
	section, ok := sectionBody(data, "github")
	if !ok {
		return GitHubConfig{}, false, nil
	}

	github := defaults
	if value, ok, err := stringSectionValue(section, "api_url"); err != nil {
		return GitHubConfig{}, true, err
	} else if ok {
		apiURL, err := validateAPIURL(value)
		if err != nil {
			return GitHubConfig{}, true, err
		}
		github.APIURL = apiURL
	}
	return github, true, nil
}

func parseOrganization(data string, defaults OrganizationConfig) (OrganizationConfig, bool, error) {
	section, ok := sectionBody(data, "organization")
	if !ok {
		return OrganizationConfig{}, false, nil
	}

	organization := defaults
	if value, ok, err := arraySectionValue(section, "policy_files"); err != nil {
		return OrganizationConfig{}, true, err
	} else if ok {
		organization.PolicyFiles = value
	}
	return organization, true, nil
}

func stripInlineComment(line string) string {
	var quote rune
	escaped := false

	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if quote == '"' && r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		if r == '#' {
			return line[:i]
		}
	}

	return line
}

func parseStringArray(input string, key string) ([]string, error) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "[") || !strings.HasSuffix(input, "]") {
		return nil, fmt.Errorf("%s must be a TOML string array", key)
	}

	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(input, "["), "]"))
	if body == "" {
		return nil, nil
	}

	var paths []string
	for len(body) > 0 {
		body = strings.TrimLeftFunc(body, unicode.IsSpace)
		if body == "" {
			break
		}

		quote := body[0]
		if quote != '"' && quote != '\'' {
			return nil, fmt.Errorf("%s must contain only strings", key)
		}

		value, tail, err := scanQuotedString(body)
		if err != nil {
			return nil, err
		}
		paths = append(paths, value)

		tail = strings.TrimLeftFunc(tail, unicode.IsSpace)
		if tail == "" {
			break
		}
		if tail[0] != ',' {
			return nil, fmt.Errorf("workflow_paths entries must be comma separated")
		}
		body = tail[1:]
	}

	return paths, nil
}

func validateAPIURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("github.api_url must not be empty")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("github.api_url is invalid: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("github.api_url must be an absolute URL")
	}
	return value, nil
}

func parseStringValue(input string, key string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("%s must be a TOML string", key)
	}

	quote := input[0]
	if quote != '"' && quote != '\'' {
		return "", fmt.Errorf("%s must be a TOML string", key)
	}

	value, tail, err := scanQuotedString(input)
	if err != nil {
		return "", fmt.Errorf("%s contains an invalid string: %w", key, err)
	}
	if strings.TrimSpace(tail) != "" {
		return "", fmt.Errorf("%s must contain exactly one string value", key)
	}
	return value, nil
}

func scanQuotedString(input string) (string, string, error) {
	if input[0] == '\'' {
		end := strings.IndexByte(input[1:], '\'')
		if end < 0 {
			return "", "", fmt.Errorf("workflow_paths contains an unterminated string")
		}
		end++
		return input[1:end], input[end+1:], nil
	}

	escaped := false
	for i := 1; i < len(input); i++ {
		switch {
		case escaped:
			escaped = false
		case input[i] == '\\':
			escaped = true
		case input[i] == '"':
			value, err := strconv.Unquote(input[:i+1])
			if err != nil {
				return "", "", fmt.Errorf("workflow_paths contains an invalid quoted string: %w", err)
			}
			return value, input[i+1:], nil
		}
	}

	return "", "", fmt.Errorf("workflow_paths contains an unterminated string")
}
