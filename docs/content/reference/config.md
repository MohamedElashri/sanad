+++
title = "Config Reference"
description = "Supported .sanad.toml keys and validation behavior."
weight = 20
template = "page"
+++

Sanad loads `.sanad.toml` by default. Pass another path with `--config`.

If `.sanad.toml` is missing, built-in defaults are used. If a non-default config path is missing or unreadable, Sanad exits with code `2`.

## Supported keys

```toml
workflow_paths = [".github/workflows"]
cooldown = "14d"

[updates]
tags = "track"
branches = "deny"
unpinned = "deny"
reusable_workflows = true

[ignore]
actions = [
  "./*",
  "docker://*"
]
files = []

[github]
api_url = "https://api.github.com"

[organization]
policy_files = []

[comments]
write = true
format = "sanad: ref={{ref}}"

[upgrade]
latest_release = "github-release"

[security]
require_full_sha = true
require_commit_in_source_repo = true
allow_private = true
deny_forks = false
```

## `workflow_paths`

Array of workflow files or directories. Directories are searched recursively for `.yml` and `.yaml` files.

## `cooldown`

Minimum age before a resolved candidate SHA can be adopted. Supports Go durations such as `48h` and day values such as `14d`.

## `[updates]`

`tags` can be `track`, `pin-current`, or `deny`.

`branches` can be `deny`, `pin-current`, or `track`.

`unpinned` can be `deny`, `default-branch`, or `latest-release`.

`reusable_workflows` controls whether reusable workflow refs are allowed.

## `[ignore]`

`actions` skips matching action refs. `files` skips matching workflow files.

Patterns use path-style glob matching, with a prefix fallback for trailing `*` patterns.

## `[github]`

`api_url` sets the GitHub API base URL. It must be an HTTPS URL without embedded credentials.

`send_token_to_custom_api_url` controls whether `GITHUB_TOKEN` or `GH_TOKEN` may be sent to a custom `api_url`. It defaults to `false`; enable it only for a trusted GitHub Enterprise endpoint.

## `[organization]`

`policy_files` lists shared config files loaded before the repository config. Repository-local values override shared policy values.

## `[comments]`

`write = false` disables inline metadata comments when the lockfile should be the only metadata source.

`format` currently accepts only `sanad: ref={{ref}}`.

## `[upgrade]`

`latest_release = "github-release"` controls `sanad upgrade --latest-release`. `release` is accepted as an alias.

## `[security]`

`require_full_sha = true` and `require_commit_in_source_repo = true` preserve Sanad's strict default behavior.

Unsupported relaxed settings fail closed. In particular, disabling full-SHA or source-repository checks is rejected, and `allow_private = false` or `deny_forks = true` are rejected until repository visibility and fork lineage checks exist.
