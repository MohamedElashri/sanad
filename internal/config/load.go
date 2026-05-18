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

	"github.com/BurntSushi/toml"
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

	if root, meta, err := decodeConfigData(data); err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	} else if meta.IsDefined("organization", "policy_files") {
		for _, policyFile := range root.Organization.PolicyFiles {
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
	raw, meta, err := decodeConfigData([]byte(data))
	if err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	}

	if meta.IsDefined("workflow_paths") {
		cfg.WorkflowPaths = raw.WorkflowPaths
	}

	if meta.IsDefined("cooldown") {
		cooldown, err := ParseDuration(raw.Cooldown)
		if err != nil {
			return Config{}, fmt.Errorf("load config %q: cooldown: %w", path, err)
		}
		cfg.Cooldown = cooldown
	}

	if meta.IsDefined("updates", "tags") {
		cfg.Updates.Tags = raw.Updates.Tags
	}
	if meta.IsDefined("updates", "branches") {
		cfg.Updates.Branches = raw.Updates.Branches
	}
	if meta.IsDefined("updates", "unpinned") {
		cfg.Updates.Unpinned = raw.Updates.Unpinned
	}
	if meta.IsDefined("updates", "reusable_workflows") {
		cfg.Updates.ReusableWorkflows = raw.Updates.ReusableWorkflows
	}

	if meta.IsDefined("ignore", "actions") {
		cfg.Ignore.Actions = raw.Ignore.Actions
	}
	if meta.IsDefined("ignore", "files") {
		cfg.Ignore.Files = raw.Ignore.Files
	}

	if meta.IsDefined("github", "api_url") {
		apiURL, err := validateAPIURL(raw.GitHub.APIURL)
		if err != nil {
			return Config{}, fmt.Errorf("load config %q: %w", path, err)
		}
		cfg.GitHub.APIURL = apiURL
	}

	if meta.IsDefined("organization", "policy_files") {
		cfg.Organization.PolicyFiles = raw.Organization.PolicyFiles
	}

	return cfg, nil
}

type fileConfig struct {
	WorkflowPaths []string               `toml:"workflow_paths"`
	Cooldown      string                 `toml:"cooldown"`
	Updates       updatesFileConfig      `toml:"updates"`
	Ignore        ignoreFileConfig       `toml:"ignore"`
	GitHub        githubFileConfig       `toml:"github"`
	Organization  organizationFileConfig `toml:"organization"`
}

type updatesFileConfig struct {
	Tags              string `toml:"tags"`
	Branches          string `toml:"branches"`
	Unpinned          string `toml:"unpinned"`
	ReusableWorkflows bool   `toml:"reusable_workflows"`
}

type ignoreFileConfig struct {
	Actions []string `toml:"actions"`
	Files   []string `toml:"files"`
}

type githubFileConfig struct {
	APIURL string `toml:"api_url"`
}

type organizationFileConfig struct {
	PolicyFiles []string `toml:"policy_files"`
}

func decodeConfigData(data []byte) (fileConfig, toml.MetaData, error) {
	var raw fileConfig
	meta, err := toml.Decode(string(data), &raw)
	if err != nil {
		return fileConfig{}, toml.MetaData{}, fmt.Errorf("parse TOML: %w", err)
	}
	return raw, meta, nil
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
