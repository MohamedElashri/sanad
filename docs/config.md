# Configuration

`sanad` loads `.sanad.toml` by default. Pass another path with `--config`.

If `.sanad.toml` is missing, the built-in defaults are used. If a non-default config path is missing or unreadable, the command fails with exit code `2`.

## Supported Keys

These keys are currently read by the CLI.

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

[security]
require_full_sha = true
require_commit_in_source_repo = true
allow_private = true
deny_forks = false
```

## `workflow_paths`

Array of workflow files or directories to scan.

Default:

```toml
workflow_paths = [".github/workflows"]
```

Directories are searched recursively for `.yml` and `.yaml` files. Missing default workflow directories are treated as empty.

Every command that scans workflows also supports an override:

```bash
sanad scan --workflows .github/workflows,other.yml
sanad plan --workflows .github/workflows
sanad check --workflows .github/workflows
sanad apply --workflows .github/workflows
```

## `cooldown`

Minimum age before a resolved candidate SHA can be adopted.

Default:

```toml
cooldown = "14d"
```

Supported duration forms include Go durations such as `48h`, `30m`, and `0s`, plus day values such as `14d`.

## `[updates]`

### `tags`

Controls tag refs such as `actions/checkout@v4`.

Allowed values:

- `track`: resolve the tag and keep tracking it through metadata.
- `pin-current`: accepted by policy and currently behaves like `track`.
- `deny`: report tag refs as policy violations.

Default:

```toml
tags = "track"
```

### `branches`

Controls branch refs such as `owner/repo@main`.

Allowed values:

- `deny`: report branch refs as policy violations.
- `pin-current`: allow the current branch head to be pinned.
- `track`: allow the branch to continue being tracked through metadata.

Default:

```toml
branches = "deny"
```

The resolver may contact GitHub before returning `error-branch-denied`, because the current implementation distinguishes tags from branches by resolving the ref.

### `unpinned`

Controls refs without `@ref`, such as `owner/repo`.

Allowed values:

- `deny`: report unpinned refs as policy violations.
- `default-branch`: resolve the repository default branch and pin its current commit.
- `latest-release`: resolve the latest GitHub release tag and pin its current commit.

Default:

```toml
unpinned = "deny"
```

If `latest-release` is configured and the repository has no releases, the action is reported as `error-unresolved`. In interactive apply mode, you can enter an explicit ref for an unpinned action or for an unpinned action whose configured discovery failed.

### `reusable_workflows`

Controls reusable workflows such as `owner/repo/.github/workflows/reuse.yml@v1`.

Default:

```toml
reusable_workflows = true
```

Set it to `false` to deny reusable workflow refs.

## `[ignore]`

Ignore rules skip matching entries from policy resolution. Ignored entries still appear in reports as skipped.

```toml
[ignore]
actions = [
  "./*",
  "docker://*",
  "owner/repo",
  "owner/repo/*",
  "owner/repo@v1"
]
files = [
  ".github/workflows/legacy.yml"
]
```

Action patterns are matched against:

- the raw `uses:` value,
- `owner/repo`,
- `owner/repo/path` when there is an action path,
- local paths or Docker refs when applicable.

Patterns use standard path-style glob matching, with a prefix fallback for trailing `*` patterns.

## `[github]`

### `api_url`

GitHub API base URL used by `plan`, `check`, and `apply`.

Default behavior uses the public GitHub API. Set `api_url` for GitHub Enterprise:

```toml
[github]
api_url = "https://github.example.com/api/v3"
```

Token lookup is unchanged: `GITHUB_TOKEN` has priority, followed by `GH_TOKEN`.

## `[organization]`

### `policy_files`

Optional shared config files to load before the repository config.

```toml
[organization]
policy_files = [
  "../security/sanad-policy.toml"
]
```

Relative paths are resolved relative to the config file that declares them. Shared policy files can set the same supported keys as `.sanad.toml`; values in the repository config override shared policy values.

## `[comments]`

### `write`

Controls whether `sanad apply` writes inline metadata comments when it rewrites workflow entries.

Default:

```toml
[comments]
write = true
```

Set this to `false` when `.github/sanad.lock.json` is the preferred metadata source:

```toml
[comments]
write = false
```

When disabled, rewrites do not add `# sanad: ref=...` comments and remove existing inline `sanad` metadata from changed lines to avoid stale metadata conflicts. `sanad apply --yes --write` still writes lockfile entries for managed pins.

### `format`

Only the built-in format is supported:

```toml
[comments]
format = "sanad: ref={{ref}}"
```

Custom formats are intentionally rejected with a config error. Inline metadata is a security-relevant input for future resolution, so `sanad` must be able to parse every comment format it writes.

## `[security]`

These keys document strict security behavior and fail closed when unsupported combinations are requested:

```toml
[security]
require_full_sha = true
require_commit_in_source_repo = true
allow_private = true
deny_forks = false
```

### `require_full_sha`

Default and only supported value:

```toml
require_full_sha = true
```

Workflow rewrites, lockfile entries, and accepted pins must use full 40-character commit SHAs. Setting this to `false` is rejected.

### `require_commit_in_source_repo`

Default and only supported value:

```toml
require_commit_in_source_repo = true
```

Resolved tags, branches, and SHA refs are verified through the referenced source repository. Setting this to `false` is rejected.

### `allow_private`

Default and only supported value:

```toml
allow_private = true
```

The current resolver can access private repositories when the supplied token allows it, but it does not inspect repository visibility. Setting this to `false` is rejected because the CLI cannot enforce it yet.

### `deny_forks`

Default and only supported value:

```toml
deny_forks = false
```

The current resolver does not inspect fork lineage. Setting this to `true` is rejected because the CLI cannot enforce it yet.

## Parser Limits

Configuration files are parsed with a TOML library. The CLI only applies the documented keys above, but ordinary TOML syntax such as comments, quoted strings, multi-line arrays, empty arrays, tables, and dotted keys is accepted for those keys.
