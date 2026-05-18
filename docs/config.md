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
- `default-branch`: reserved by policy, but not fully resolved by the current CLI.
- `latest-release`: reserved by policy, but not fully resolved by the current CLI.

Default:

```toml
unpinned = "deny"
```

In interactive apply mode, you can enter an explicit ref for an unpinned action.

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

## Reserved Sections

The example config includes sections that document intended future behavior but are not currently read by the CLI:

```toml
[comments]
write = true
format = "sanad: ref={{ref}}"

[security]
require_full_sha = true
require_commit_in_source_repo = true
allow_private = true
deny_forks = false
```

Current behavior:

- Inline comments are always written for workflow rewrites.
- Full SHA and source-repository checks are enforced by implemented policy and resolver behavior, not by `[security]` config.

## Parser Limits

Configuration files are parsed with a TOML library. The CLI only applies the documented keys above, but ordinary TOML syntax such as comments, quoted strings, multi-line arrays, empty arrays, tables, and dotted keys is accepted for those keys.
