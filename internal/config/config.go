package config

import "time"

const DefaultPath = ".sanad.toml"

type Config struct {
	Source        string
	WorkflowPaths []string
	Cooldown      time.Duration
	Updates       UpdatesConfig
	Ignore        IgnoreConfig
	GitHub        GitHubConfig
	Organization  OrganizationConfig
}

type UpdatesConfig struct {
	Tags              string
	Branches          string
	Unpinned          string
	ReusableWorkflows bool
}

type IgnoreConfig struct {
	Actions []string
	Files   []string
}

type GitHubConfig struct {
	APIURL string
}

type OrganizationConfig struct {
	PolicyFiles []string
}

func Default() Config {
	return Config{
		Source:        "defaults",
		WorkflowPaths: []string{".github/workflows"},
		Cooldown:      14 * 24 * time.Hour,
		Updates: UpdatesConfig{
			Tags:              "track",
			Branches:          "deny",
			Unpinned:          "deny",
			ReusableWorkflows: true,
		},
		Ignore: IgnoreConfig{
			Actions: []string{"./*", "docker://*"},
		},
	}
}
