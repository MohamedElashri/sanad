+++
title = "CLI Reference"
description = "Commands, flags, outputs, and exit codes."
weight = 10
template = "page"
+++

Sanad exposes five top-level command groups:

```bash
sanad audit
sanad update
sanad lock
sanad completion
sanad version
```

Legacy top-level workflow commands remain available as hidden compatibility aliases for one migration period:

| Old command | New command |
| --- | --- |
| `sanad scan` | `sanad audit scan` |
| `sanad plan` | `sanad audit plan` |
| `sanad check` | `sanad audit check` |
| `sanad apply` | `sanad update apply` |
| `sanad upgrade` | `sanad update upgrade` |

Global flags:

```bash
--config string   path to config file (default ".sanad.toml")
--format string   output format: table, json, or command-specific formats (default "table")
--color string    colorize human output: auto, always, or never (default "auto")
```

`--color auto` enables ANSI color only for capable terminals. `--color never`, `NO_COLOR`, `CLICOLOR=0`, or `SANAD_COLOR=never` disable color; `--color always`, `CLICOLOR_FORCE=1`, or `SANAD_COLOR=always` force it. JSON, SARIF, and generated Markdown outputs are never colorized.

Sanad uses standard ANSI colors that stay readable on typical dark and light terminal backgrounds. If your terminal exposes `COLORFGBG`, sanad uses it to tune warning colors; `SANAD_COLOR_THEME=dark` or `SANAD_COLOR_THEME=light` can override that detection.

## `audit scan`

Discover and classify `uses:` entries without contacting GitHub.

```bash
sanad audit scan
sanad --format json audit scan
sanad audit scan --workflows .github/workflows
```

Alias: `sanad scan`.

## `audit plan`

Resolve actionable refs and show decisions without writing files.

```bash
GITHUB_TOKEN=$(gh auth token) sanad audit plan
GITHUB_TOKEN=$(gh auth token) sanad --format json audit plan
GITHUB_TOKEN=$(gh auth token) sanad audit plan --out sanad-plan.json
GITHUB_TOKEN=$(gh auth token) sanad audit plan --pr-body-out sanad-pr-body.md
```

Decision values include `unchanged`, `update`, `pending-cooldown`, skip decisions, and policy errors such as `error-unpinned`, `error-short-sha`, `error-tag-denied`, `error-branch-denied`, and `error-unresolved`.

Alias: `sanad plan`.

## `audit check`

Validate workflows against policy.

```bash
GITHUB_TOKEN=$(gh auth token) sanad audit check
GITHUB_TOKEN=$(gh auth token) sanad --format json audit check
GITHUB_TOKEN=$(gh auth token) sanad audit check --format sarif
GITHUB_TOKEN=$(gh auth token) sanad audit check --allow-pending-cooldown
```

Default behavior fails on policy violations, mutable refs that still need to be pinned, and managed pins whose resolved candidate differs from the workflow SHA. Pass `--allow-pending-cooldown` to allow cooldown-pending managed candidates while still failing eligible updates.

Alias: `sanad check`.

## `update apply`

Apply approved updates to workflow files and refresh `.github/sanad.lock.json`.

```bash
GITHUB_TOKEN=$(gh auth token) sanad update apply --dry-run
GITHUB_TOKEN=$(gh auth token) sanad update apply --interactive
GITHUB_TOKEN=$(gh auth token) sanad update apply --yes --write
```

`--dry-run` prints a unified diff and writes nothing. Non-interactive writes require `--yes --write`.

Alias: `sanad apply`.

## `update upgrade`

Move managed full-SHA pins from one logical ref to another.

```bash
GITHUB_TOKEN=$(gh auth token) sanad update upgrade
GITHUB_TOKEN=$(gh auth token) sanad update upgrade --action actions/checkout --to v5
GITHUB_TOKEN=$(gh auth token) sanad update upgrade --all --level minor --dry-run
GITHUB_TOKEN=$(gh auth token) sanad update upgrade --all --constraint '< 6' --write
GITHUB_TOKEN=$(gh auth token) sanad update upgrade --all --selection latest
```

With no selector, `upgrade` scans all managed pins. With no explicit `--to`, it selects stable GitHub releases using the configured policy. The defaults allow major upgrades and choose the highest release that has satisfied cooldown. The command is dry-run by default; add `--write` after reviewing the diff.

`--level major|minor|patch` and `--constraint <range>` are mutually exclusive. `--selection latest-eligible|latest` controls cooldown fallback. `--to <ref>` bypasses automatic SemVer selection but still enforces cooldown and cannot be combined with automatic policy flags.

`--latest-release` remains as a compatibility alias for `--selection latest`; `--latest-release-mode` is deprecated. Automatic selection rejects non-SemVer current refs, drafts, prereleases, and non-SemVer release tags. Use `--to` for those refs.

Alias: `sanad upgrade`.

## `lock status`

Compare `.github/sanad.lock.json` with the current workflow `uses:` nodes and report matched, stale, repairable, and blocking entries.

```bash
sanad lock status
sanad --format json lock status
sanad lock status --workflows .github/workflows
```

`status` does not write files and does not contact GitHub. Human output includes a summary, diagnostics, and planned lockfile changes when repair or refresh would modify the lockfile. JSON output includes `summary`, `entries`, `diagnostics`, and `changes` fields for CI comments or bots.

## `lock refresh`

Regenerate lock entries for current managed workflow pins without changing workflow files.

```bash
sanad lock refresh --dry-run
sanad lock refresh --write
sanad --format json lock refresh --dry-run
```

Refresh is dry-run unless `--write` is present. It rebuilds the active managed entry set from current pinned workflow nodes, removing entries no longer represented in the refreshed scope. With `--workflows`, entries outside the requested files or directories are preserved.

## `lock repair`

Apply safe reconciliation fixes to the lockfile without changing workflow files.

```bash
sanad lock repair --dry-run
sanad lock repair --write
sanad --format json lock repair --dry-run
```

Repair updates stale action identity, logical ref, and pinned SHA fields when the current workflow and inline `sanad` metadata make the intended state clear. It preserves missing-node and out-of-scope entries, as well as compatible candidate history. Use `lock prune` when deletion is intended.

Malformed JSON is reported as an explicit lockfile load error. Unsupported lockfile versions, invalid SHA fields, invalid timestamps, duplicate entries, and invalid inline comments remain blocking diagnostics. Write mode exits without changing the lockfile while blocking diagnostics are present.

## `lock prune`

Remove lockfile entries for deleted workflow nodes only.

```bash
sanad lock prune --dry-run
sanad lock prune --write
```

Use prune when you explicitly want to drop entries for missing workflow nodes and leave other stale active entries untouched. With `--workflows`, only missing entries inside that scope are removed.

## `version`

Print build metadata:

```bash
sanad version
```

## `completion`

Generate or install shell completions:

```bash
sanad completion bash
sanad completion zsh
sanad completion fish
sanad completion powershell
sanad completion install
sanad completion install zsh
```

`sanad completion install` detects the current shell from the environment and installs completions for the current user. Pass a shell name when detection is not possible. Homebrew and Nix installs include completions automatically.

For manual installs, `completion install` writes completion files under the current user's shell config/data directories. It updates bash, zsh, and PowerShell profile files with a marked activation block when needed. Use `--dry-run` to preview the paths, or `--no-profile` to write only the completion file.

## Exit codes

```text
0  success
1  policy violation or changes needed in check/apply mode
2  invalid configuration
3  unresolved action reference
4  GitHub API failure
5  rate limit failure
6  unsafe rewrite prevented
7  file system write failure
8  internal error
```
