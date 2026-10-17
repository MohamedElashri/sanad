package sanad

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"
)

func PrintConfig(ctx context.Context, opts Options, out io.Writer) error {
	root, err := Root(ctx, opts.Root)
	if err != nil {
		return err
	}
	name := opts.Config
	if name == "" {
		name = ConfigPath
	}
	if _, err := safePath(root, name); err != nil {
		return err
	}
	cfg, err := loadConfig(root, name)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(cfg)
}

const ConfigPath = ".sanad.toml"
const StatePath = ".github/sanad/state.json"

type UpdatePolicy struct {
	Level      string `toml:"level" json:"level,omitempty"`
	Constraint string `toml:"constraint" json:"constraint,omitempty"`
}

type Config struct {
	Version int `toml:"version" json:"version"`
	Native  struct {
		Enabled bool `toml:"enabled" json:"enabled"`
	} `toml:"native" json:"native"`
	Update      UpdatePolicy `toml:"update" json:"update"`
	Observation struct {
		MinimumAge string `toml:"minimum_age" json:"minimum_age,omitempty"`
	} `toml:"observation" json:"observation"`
	Actions map[string]UpdatePolicy `toml:"actions" json:"actions,omitempty"`
}

func DefaultConfig() Config {
	c := Config{Version: 1, Update: UpdatePolicy{Level: "minor"}}
	c.Actions = map[string]UpdatePolicy{}
	return c
}

func loadConfig(root, name string) (Config, error) {
	c := DefaultConfig()
	b, err := os.ReadFile(filepath.Join(root, name))
	if os.IsNotExist(err) && name == ConfigPath {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	var raw Config
	md, err := toml.Decode(string(b), &raw)
	if err != nil {
		return c, fmt.Errorf("%s: %w", name, err)
	}
	if raw.Version != 1 {
		return c, fmt.Errorf("%s: expected version = 1; remove 0.x configuration and .github/sanad.lock.json, restore desired uses refs, then run sanad init", name)
	}
	if len(md.Undecoded()) != 0 {
		return c, fmt.Errorf("%s: unknown configuration key %s", name, md.Undecoded()[0])
	}
	if raw.Update.Level == "" && raw.Update.Constraint == "" {
		raw.Update.Level = "minor"
	}
	if err := validatePolicy(raw.Update); err != nil {
		return c, err
	}
	for action, policy := range raw.Actions {
		if len(strings.Split(action, "/")) < 2 || strings.ContainsAny(action, "@\n\r") {
			return c, fmt.Errorf("invalid action policy %q", action)
		}
		if err := validatePolicy(policy); err != nil {
			return c, fmt.Errorf("%s: %w", action, err)
		}
	}
	if _, err := duration(raw.Observation.MinimumAge); err != nil {
		return c, err
	}
	return raw, nil
}

func validatePolicy(p UpdatePolicy) error {
	if p.Level != "" && p.Constraint != "" {
		return fmt.Errorf("level and constraint are mutually exclusive")
	}
	if p.Level != "" && p.Level != "patch" && p.Level != "minor" && p.Level != "major" {
		return fmt.Errorf("invalid update level %q", p.Level)
	}
	if p.Constraint != "" {
		if _, err := semver.NewConstraint(p.Constraint); err != nil {
			return err
		}
	}
	return nil
}

func duration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	if strings.HasSuffix(s, "d") {
		s = strings.TrimSuffix(s, "d") + "h"
		d, err := time.ParseDuration(s)
		if err != nil || d < 0 || d > time.Duration(1<<63-1)/24 {
			return 0, fmt.Errorf("invalid duration")
		}
		return d * 24, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("invalid nonnegative duration %q", s)
	}
	return d, nil
}

func releaseCooldown(root string) (time.Duration, error) {
	var b []byte
	for _, name := range []string{".github/dependabot.yml", ".github/dependabot.yaml"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return 0, err
		}
		b = data
		break
	}
	if len(b) == 0 {
		return 0, nil
	}
	var config struct {
		Updates []struct {
			Ecosystem string         `yaml:"package-ecosystem"`
			Cooldown  map[string]any `yaml:"cooldown"`
		} `yaml:"updates"`
	}
	if err := yaml.Unmarshal(b, &config); err != nil {
		return 0, fmt.Errorf("invalid Dependabot configuration: %w", err)
	}
	found := false
	var days int
	for _, entry := range config.Updates {
		if entry.Ecosystem != "github-actions" {
			continue
		}
		if found {
			return 0, fmt.Errorf("multiple github-actions Dependabot entries are ambiguous; use one repository-wide entry")
		}
		found = true
		for key, value := range entry.Cooldown {
			if key != "default-days" {
				return 0, fmt.Errorf("unsupported Dependabot cooldown.%s; use default-days", key)
			}
			n, ok := value.(int)
			if !ok || n < 0 || n > 36500 {
				return 0, fmt.Errorf("invalid Dependabot cooldown.default-days")
			}
			days = n
		}
	}
	return time.Duration(days) * 24 * time.Hour, nil
}
