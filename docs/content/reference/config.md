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
cooldown_source = "source"

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

[organization]
policy_files = []

[comments]
write = true
format = "sanad: ref={{ref}}"

[upgrade]
level = "major"
selection = "latest-eligible"

[upgrade.actions."actions/checkout"]
constraint = ">= 4, < 6"

[security]
require_full_sha = true
require_commit_in_source_repo = true
allow_private = true
deny_forks = false
```

## `workflow_paths`

Array of workflow files or directories. Directories are searched recursively for `.yml` and `.yaml` files. Paths must be relative and must not escape the repository root with `..`.

## `cooldown`

Minimum age before a resolved candidate SHA can be adopted. Supports Go durations such as `48h` and day values such as `14d`.

## `cooldown_source`

Controls which timestamp feeds cooldown evaluation:

`source` uses upstream release, tag, or commit timestamps. This is the default.

`first-seen` uses the time Sanad first recorded a candidate SHA locally in `.github/sanad.lock.json`.

## `[updates]`

`tags` can be `track`, `pin-current`, or `deny`.

`branches` can be `deny`, `pin-current`, or `track`.

`unpinned` can be `deny`, `default-branch`, or `latest-release`.

`reusable_workflows` controls whether reusable workflow refs are allowed.

## `[ignore]`

`actions` skips matching action refs. `files` skips matching workflow files.

Patterns use path-style glob matching, with a prefix fallback for trailing `*` patterns.

## `[organization]`

`policy_files` lists shared config files loaded before the repository config. Repository-local values override shared policy values.

## `[comments]`

`write = false` disables inline metadata comments when the lockfile should be the only metadata source.

`format` currently accepts only `sanad: ref={{ref}}`.

## `[upgrade]`

`level` sets the maximum automatic SemVer change: `patch` stays within the current major and minor, `minor` stays within the current major, and `major` permits any newer stable release. The default is `major`.

`constraint` accepts a SemVer constraint such as `">= 4, < 6"` and is mutually exclusive with `level`.

`selection = "latest-eligible"` chooses the highest matching release that has satisfied cooldown. `selection = "latest"` evaluates only the highest matching release and waits if it is cooling down.

`[upgrade.actions."owner/repo[/path]"]` applies `level`, `constraint`, and `selection` overrides to one action. CLI flags override per-action settings, which override global settings.

The deprecated `latest_release = "github-release"` key and its `release` alias remain accepted for compatibility.

## `[security]`

`require_full_sha = true` and `require_commit_in_source_repo = true` preserve Sanad's strict default behavior.

Unsupported relaxed settings fail closed. In particular, disabling full-SHA or source-repository checks is rejected, and `allow_private = false` or `deny_forks = true` are rejected until repository visibility and fork lineage checks exist.
