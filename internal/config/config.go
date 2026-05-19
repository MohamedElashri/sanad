package config

import "time"

const DefaultPath = ".sanad.toml"
const DefaultCommentFormat = "sanad: ref={{ref}}"
const DefaultUpgradeLatestRelease = "github-release"
const DefaultCooldownSource = "source"

type Config struct {
	Source         string
	WorkflowPaths  []string
	Cooldown       time.Duration
	CooldownSource string
	Updates        UpdatesConfig
	Ignore         IgnoreConfig
	Organization   OrganizationConfig
	Comments       CommentsConfig
	Security       SecurityConfig
	Upgrade        UpgradeConfig
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

type OrganizationConfig struct {
	PolicyFiles []string
}

type CommentsConfig struct {
	Write  bool
	Format string
}

type SecurityConfig struct {
	RequireFullSHA            bool
	RequireCommitInSourceRepo bool
	AllowPrivate              bool
	DenyForks                 bool
}

type UpgradeConfig struct {
	LatestRelease string
}

func Default() Config {
	return Config{
		Source:         "defaults",
		WorkflowPaths:  []string{".github/workflows"},
		Cooldown:       14 * 24 * time.Hour,
		CooldownSource: DefaultCooldownSource,
		Updates: UpdatesConfig{
			Tags:              "track",
			Branches:          "deny",
			Unpinned:          "deny",
			ReusableWorkflows: true,
		},
		Ignore: IgnoreConfig{
			Actions: []string{"./*", "docker://*"},
		},
		Comments: CommentsConfig{
			Write:  true,
			Format: DefaultCommentFormat,
		},
		Security: SecurityConfig{
			RequireFullSHA:            true,
			RequireCommitInSourceRepo: true,
			AllowPrivate:              true,
			DenyForks:                 false,
		},
		Upgrade: UpgradeConfig{
			LatestRelease: DefaultUpgradeLatestRelease,
		},
	}
}
