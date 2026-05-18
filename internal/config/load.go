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

	if meta.IsDefined("comments", "write") {
		cfg.Comments.Write = raw.Comments.Write
	}
	if meta.IsDefined("comments", "format") {
		if err := validateCommentFormat(raw.Comments.Format); err != nil {
			return Config{}, fmt.Errorf("load config %q: %w", path, err)
		}
		cfg.Comments.Format = raw.Comments.Format
	}

	if meta.IsDefined("security", "require_full_sha") {
		cfg.Security.RequireFullSHA = raw.Security.RequireFullSHA
	}
	if meta.IsDefined("security", "require_commit_in_source_repo") {
		cfg.Security.RequireCommitInSourceRepo = raw.Security.RequireCommitInSourceRepo
	}
	if meta.IsDefined("security", "allow_private") {
		cfg.Security.AllowPrivate = raw.Security.AllowPrivate
	}
	if meta.IsDefined("security", "deny_forks") {
		cfg.Security.DenyForks = raw.Security.DenyForks
	}
	if err := validateSecurityConfig(cfg.Security); err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	}

	if meta.IsDefined("upgrade", "latest_release") {
		latestRelease, err := normalizeUpgradeLatestRelease(raw.Upgrade.LatestRelease)
		if err != nil {
			return Config{}, fmt.Errorf("load config %q: %w", path, err)
		}
		cfg.Upgrade.LatestRelease = latestRelease
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
	Comments      commentsFileConfig     `toml:"comments"`
	Security      securityFileConfig     `toml:"security"`
	Upgrade       upgradeFileConfig      `toml:"upgrade"`
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

type commentsFileConfig struct {
	Write  bool   `toml:"write"`
	Format string `toml:"format"`
}

type securityFileConfig struct {
	RequireFullSHA            bool `toml:"require_full_sha"`
	RequireCommitInSourceRepo bool `toml:"require_commit_in_source_repo"`
	AllowPrivate              bool `toml:"allow_private"`
	DenyForks                 bool `toml:"deny_forks"`
}

type upgradeFileConfig struct {
	LatestRelease string `toml:"latest_release"`
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

func validateCommentFormat(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("comments.format must not be empty")
	}
	if value != DefaultCommentFormat {
		return fmt.Errorf("comments.format %q is not supported; expected %q because sanad must be able to parse its own metadata safely", value, DefaultCommentFormat)
	}
	return nil
}

func validateSecurityConfig(cfg SecurityConfig) error {
	if !cfg.RequireFullSHA {
		return fmt.Errorf("security.require_full_sha=false is not supported; sanad always requires full 40-character SHAs")
	}
	if !cfg.RequireCommitInSourceRepo {
		return fmt.Errorf("security.require_commit_in_source_repo=false is not supported; sanad resolves and verifies commits in the referenced source repository")
	}
	if !cfg.AllowPrivate {
		return fmt.Errorf("security.allow_private=false is not supported because repository visibility is not resolved by the current CLI")
	}
	if cfg.DenyForks {
		return fmt.Errorf("security.deny_forks=true is not supported because fork lineage is not resolved by the current CLI")
	}
	return nil
}

func normalizeUpgradeLatestRelease(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch value {
	case DefaultUpgradeLatestRelease, "release":
		return DefaultUpgradeLatestRelease, nil
	case "":
		return "", fmt.Errorf("upgrade.latest_release must not be empty")
	default:
		return "", fmt.Errorf("upgrade.latest_release %q is not supported; expected %q", value, DefaultUpgradeLatestRelease)
	}
}
