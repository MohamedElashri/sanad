+++
title = "Configuration"
description = "Configure workflow discovery, update policy, cooldown, metadata comments, and GitHub API access."
weight = 20
template = "page"
+++

Sanad loads `.sanad.toml` by default. If that file is missing, built-in defaults are used.

Start with the defaults and add config only for policy choices your repository needs:

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

[comments]
write = true
format = "sanad: ref={{ref}}"

[security]
require_full_sha = true
require_commit_in_source_repo = true
allow_private = true
deny_forks = false
```

## Common settings

`workflow_paths` controls which workflow files or directories are scanned.

`cooldown` controls how old a newly resolved candidate commit must be before Sanad can adopt it. The default is `14d`.

`updates.tags = "track"` means refs like `actions/checkout@v4` are resolved, pinned, and tracked through metadata.

`updates.branches = "deny"` is the default because branches move more often than release tags. Interactive apply can pin a branch head and optionally persist branch tracking in config.

`updates.unpinned = "deny"` is the default. Set it to `default-branch` or `latest-release` only when your repository policy intentionally allows Sanad to discover a target for unpinned `owner/repo` actions.

`comments.write = false` disables inline `sanad: ref=...` comments. The lockfile remains the metadata source.

## GitHub Enterprise

Set the API endpoint for GitHub Enterprise:

```toml
[github]
api_url = "https://github.example.com/api/v3"
```

Token lookup is unchanged: `GITHUB_TOKEN` has priority, then `GH_TOKEN`.

For all supported keys and validation behavior, see [Config Reference](../../reference/config/).
