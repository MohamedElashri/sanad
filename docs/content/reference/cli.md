+++
title = "CLI Reference"
description = "Commands, flags, outputs, and exit codes."
weight = 10
template = "page"
+++

Sanad has five workflow commands and one metadata command:

```bash
sanad scan
sanad plan
sanad check
sanad apply
sanad upgrade
sanad version
```

Global flags:

```bash
--config string   path to config file (default ".sanad.toml")
--format string   output format: table, json, or command-specific formats (default "table")
--color string    colorize human output: auto, always, or never (default "auto")
```

`--color auto` enables ANSI color only for capable terminals. `--color never`, `NO_COLOR`, `CLICOLOR=0`, or `SANAD_COLOR=never` disable color; `--color always`, `CLICOLOR_FORCE=1`, or `SANAD_COLOR=always` force it. JSON, SARIF, and generated Markdown outputs are never colorized.

Sanad uses standard ANSI colors that stay readable on typical dark and light terminal backgrounds. If your terminal exposes `COLORFGBG`, sanad uses it to tune warning colors; `SANAD_COLOR_THEME=dark` or `SANAD_COLOR_THEME=light` can override that detection.

## `scan`

Discover and classify `uses:` entries without contacting GitHub.

```bash
sanad scan
sanad --format json scan
sanad scan --workflows .github/workflows
```

## `plan`

Resolve actionable refs and show decisions without writing files.

```bash
GITHUB_TOKEN=$(gh auth token) sanad plan
GITHUB_TOKEN=$(gh auth token) sanad --format json plan
GITHUB_TOKEN=$(gh auth token) sanad plan --out sanad-plan.json
GITHUB_TOKEN=$(gh auth token) sanad plan --pr-body-out sanad-pr-body.md
```

Decision values include `unchanged`, `update`, `pending-cooldown`, skip decisions, and policy errors such as `error-unpinned`, `error-short-sha`, `error-tag-denied`, `error-branch-denied`, and `error-unresolved`.

## `check`

Validate workflows against policy.

```bash
GITHUB_TOKEN=$(gh auth token) sanad check
GITHUB_TOKEN=$(gh auth token) sanad --format json check
GITHUB_TOKEN=$(gh auth token) sanad check --format sarif
GITHUB_TOKEN=$(gh auth token) sanad check --strict
GITHUB_TOKEN=$(gh auth token) sanad check --fail-on-updates
GITHUB_TOKEN=$(gh auth token) sanad check --strict --allow-pending-cooldown
```

Default behavior fails on policy violations and mutable refs that still need to be pinned. `--strict` also fails on eligible or cooldown-pending managed updates, unless `--allow-pending-cooldown` is passed.

## `apply`

Apply approved updates to workflow files and refresh `.github/sanad.lock.json`.

```bash
GITHUB_TOKEN=$(gh auth token) sanad apply --dry-run
GITHUB_TOKEN=$(gh auth token) sanad apply --interactive
GITHUB_TOKEN=$(gh auth token) sanad apply --yes --write
```

`--dry-run` prints a unified diff and writes nothing. Non-interactive writes require `--yes --write`.

## `upgrade`

Move managed full-SHA pins from one logical ref to another.

```bash
GITHUB_TOKEN=$(gh auth token) sanad upgrade --action actions/checkout --to v5
GITHUB_TOKEN=$(gh auth token) sanad upgrade --action actions/setup-go --to v6 --write
GITHUB_TOKEN=$(gh auth token) sanad upgrade --all --latest-release --dry-run
```

Use exactly one target selector: `--to <ref>` or `--latest-release`.

## `version`

Print build metadata:

```bash
sanad version
```

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
