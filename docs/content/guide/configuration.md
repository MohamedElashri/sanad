+++
title = "Configuration"
description = "Configure workflow discovery, update policy, cooldown, metadata comments, and strict security behavior."
weight = 20
template = "page"
+++

Sanad loads `.sanad.toml` by default. If that file is missing, built-in defaults are used.

Start with the defaults and add config only for policy choices your repository needs:

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

`workflow_paths` controls which workflow files or directories are scanned. Configured paths must be relative and stay inside the repository root.

`cooldown` controls how old a resolved candidate must be before Sanad can adopt it. The default is `14d`.

`cooldown_source = "source"` uses upstream release, tag, or commit timestamps and is the default. Set `cooldown_source = "first-seen"` to use the time Sanad first recorded a candidate SHA locally.

`updates.tags = "track"` means refs like `actions/checkout@v4` are resolved, pinned, and tracked through metadata.

`updates.branches = "deny"` is the default because branches move more often than release tags. Interactive apply can pin a branch head and optionally persist branch tracking in config.

`updates.unpinned = "deny"` is the default. Set it to `default-branch` or `latest-release` only when your repository policy intentionally allows Sanad to discover a target for unpinned `owner/repo` actions.

`comments.write = false` disables inline `sanad: ref=...` comments. The lockfile remains the metadata source.

## GitHub API

Sanad resolves refs through `github.com` only. Custom GitHub API endpoints are intentionally unsupported so repository config cannot redirect CI credentials to another host.

For all supported keys and validation behavior, see [Config Reference](../../reference/config/).
