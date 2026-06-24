<div align="center">
  <img src="docs/static/logo.svg" alt="Sanad logo" width="128" height="128">
  <h1>Sanad</h1>
  <p><strong>Pin GitHub Actions to immutable SHAs, then keep the refs you trust moving.</strong></p>
  <p>
    <a href="https://github.com/MohamedElashri/sanad/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/MohamedElashri/sanad/actions/workflows/ci.yml/badge.svg"></a>
    <a href="https://github.com/MohamedElashri/sanad/actions/workflows/pages.yml"><img alt="Docs" src="https://github.com/MohamedElashri/sanad/actions/workflows/pages.yml/badge.svg?label=docs"></a>
    <a href="https://github.com/MohamedElashri/sanad/actions/workflows/release.yml"><img alt="Release workflow" src="https://github.com/MohamedElashri/sanad/actions/workflows/release.yml/badge.svg"></a>
    <a href="https://github.com/MohamedElashri/sanad/releases"><img alt="Latest release" src="https://img.shields.io/github/v/release/MohamedElashri/sanad?sort=semver&display_name=tag&logo=github"></a>
    <a href="https://pkg.go.dev/github.com/MohamedElashri/sanad"><img alt="Go reference" src="https://pkg.go.dev/badge/github.com/MohamedElashri/sanad.svg"></a>
    <a href="https://goreportcard.com/report/github.com/MohamedElashri/sanad"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/MohamedElashri/sanad"></a>
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/github/license/MohamedElashri/sanad"></a>
  </p>
  <p>
    <a href="https://melashri.net/sanad/">Documentation</a>
    |
    <a href="#installation">Installation</a>
    |
    <a href="#quickstart">Quickstart</a>
    |
    <a href="docs/content/reference/cli.md">CLI reference</a>
    |
    <a href="docs/content/advanced/security-model.md">Security model</a>
  </p>
</div>

**sanad** pins and updates GitHub Actions dependencies to immutable commit SHAs while preserving the logical refs you want to track.

## Installation

### Homebrew

On macOS or Linux with Homebrew:

```bash
brew tap MohamedElashri/sanad && brew install sanad
```
Check that it is installed 

```bash
sanad version
```

Homebrew installs shell completions for bash, zsh, fish, and PowerShell automatically.

### Nix

Run the packaged release directly:

```bash
nix run github:MohamedElashri/sanad -- version
```

Or install it into your profile:

```bash
nix profile install github:MohamedElashri/sanad
```
Check that it is installed 

```bash
sanad version
```

The flake installs the published release archive for your platform and verifies it with the release checksum.
The Nix package installs bash, zsh, and fish completions automatically.

### Go

Install the latest tagged release with Go:

```bash
go install github.com/MohamedElashri/sanad/cmd/sanad@latest
```

### Prebuilt archives

Tagged releases also publish Linux, macOS, and Windows archives on GitHub Releases. Download the archive for your platform, place the `sanad` binary on your `PATH`, and verify it against the published `sanad_<version>_checksums.txt` file.

Check the installed build:

```bash
sanad version
```

For manual archive or `go install` usage, install completions for your current shell with:

```bash
sanad completion install
```

Sanad detects bash, zsh, fish, and PowerShell from your environment. You can also pass the shell explicitly:

```bash
sanad completion install bash
sanad completion install zsh
sanad completion install fish
sanad completion install powershell
```

Use `sanad completion install --dry-run` to preview the files that would be written, or `--no-profile` to install the completion file without updating shell profile files.

## Quickstart

Scan workflows without making network calls:

```bash
sanad audit scan
```

Preview policy decisions and proposed pin updates:

```bash
GITHUB_TOKEN=$(gh auth token) sanad audit plan
```

Preview updates without changing files:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update apply --dry-run
```

Add `--diff` when you want the unified file patch:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update apply --dry-run --diff
```

Apply locally:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update apply --yes --write
```

Validate locally:

```bash
GITHUB_TOKEN=$(gh auth token) sanad audit check
```

If Dependabot or a manual edit changes a pinned workflow entry and leaves `.github/sanad.lock.json` stale, inspect and repair the lockfile without deleting it:

```bash
sanad lock status
sanad lock repair --dry-run
sanad lock repair --write
```

`sanad audit plan`, `sanad audit check`, `sanad update apply`, and `sanad update upgrade` may contact GitHub when resolution is needed. `sanad audit scan` and `sanad lock status|refresh|repair|prune` are local-only.

Human-readable output uses color automatically when the terminal supports it. Use `--color never` or `NO_COLOR=1` to disable color, and `--color always` to force it for pagers or demos.

## Example

Before:

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
```

After:

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # sanad: ref=v4
      - uses: actions/setup-go@93397bea11091df50f3d7e59dc26a7711a8bcfbe # sanad: ref=v5
```

The workflow executes immutable SHAs. The comments and lockfile tell `sanad` which logical refs to resolve on future runs.

## Scope

Sanad scans workflow files under `.github/workflows` by default, classifies `uses:` references, resolves GitHub tags and branches through the GitHub API, rewrites mutable action refs to full SHAs, adds `# sanad: ref=...` metadata, maintains `.github/sanad.lock.json`, applies cooldown rules, and emits table, JSON, SARIF, and Markdown helper output.

It is not a general dependency updater, vulnerability scanner, workflow formatter, YAML linter, Docker image updater, local action rewriter, or dedicated GitHub Action wrapper.

In Arabic scholarly culture, a sanad is a chain of transmission back to a source. This tool keeps that chain explicit for workflow dependencies: the workflow runs an immutable commit, while metadata records the tag or branch that commit came from.

## Configuration

Create `.sanad.toml` when the defaults are not enough:

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

[upgrade]
level = "major"
selection = "latest-eligible"

[security]
require_full_sha = true
require_commit_in_source_repo = true
allow_private = true
deny_forks = false
```

Set `[comments].write = false` to rely on `.github/sanad.lock.json` without inline `sanad` comments. See the [config reference](docs/content/reference/config.md) for exact supported keys.

Interactive apply can optionally persist branch tracking by writing `[updates].branches = "track"` after final confirmation.

## Commands

```bash
sanad audit scan
sanad audit plan
sanad audit check
sanad update apply
sanad update upgrade
sanad lock status
sanad lock refresh
sanad lock repair
sanad lock prune
sanad completion
sanad version
```

All commands accept:

```bash
--config .sanad.toml
--format table
--format json
```

Legacy top-level invocations are still accepted as hidden compatibility aliases for one migration period:

| Old command | New command |
| --- | --- |
| `sanad scan` | `sanad audit scan` |
| `sanad plan` | `sanad audit plan` |
| `sanad check` | `sanad audit check` |
| `sanad apply` | `sanad update apply` |
| `sanad upgrade` | `sanad update upgrade` |

`sanad audit check --format sarif` emits SARIF for code scanning, and `sanad audit plan --pr-body-out body.md` writes a Markdown pull request summary for automation.

`sanad update upgrade` previews the highest stable SemVer release allowed by policy and cooldown for every managed pin. It is dry-run by default and does not print file patches unless `--diff` is passed; add `--write` to apply the reported upgrades.

Use `--level minor|patch`, `--constraint '< 6'`, or matching `[upgrade]` configuration to restrict automatic upgrades. `--selection latest` restores the wait-for-the-newest behavior; the default `latest-eligible` can select an older release while a newer one is cooling down.

`sanad update upgrade --action actions/checkout --to v5` intentionally moves one managed pin to a specific logical ref while keeping workflow execution pinned to a full SHA.

`sanad lock status` reports stale, repairable, and blocking lockfile diagnostics. Use `sanad lock refresh --write` to rebuild active managed entries, `sanad lock repair --write` for non-destructive reconciliation fixes, and `sanad lock prune --write` for explicit removal of entries belonging to deleted workflow nodes. Audit commands are read-only, and scoped writes preserve entries outside `--workflows`.

Command-specific usage is covered in the [CLI reference](docs/content/reference/cli.md).

## GitHub Authentication

`sanad` reads tokens from environment variables:

1. `GITHUB_TOKEN`
2. `GH_TOKEN`

Tokens are used for GitHub API requests and are never printed by the CLI. Public repositories can work without a token, but authenticated requests are strongly recommended for CI and private repositories.

For local shell usage, prefer reusing the GitHub CLI token instead of pasting a token into your terminal:

```bash
GITHUB_TOKEN=$(gh auth token) sanad audit plan
```


## Security Model

The core policy is simple: workflow dependencies should run immutable full-length SHAs. Mutable tags and branches are resolved to commits, short SHAs are rejected, local and Docker actions are skipped by default, and branch or unpinned behavior must be explicitly allowed before it is managed non-interactively.

See the [security model](docs/content/advanced/security-model.md) for the full model.

## Cooldown

The default cooldown is `14d`, and the default `cooldown_source = "source"` uses the upstream release, tag, or commit timestamp. Automatic upgrades select the highest matching release that has satisfied this window. Set `cooldown_source = "first-seen"` for the stricter mode: Sanad records candidate histories in the lockfile and waits for the local observation window before adopting them. Run upgrade with `--write` to persist observations even when no workflow update is yet eligible.

## CI

Example enforcement job:

```yaml
name: Check pinned actions

on:
  pull_request:
  push:
    branches: [main]

jobs:
  sanad:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd
      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff
        with:
          go-version: "1.26.x"
      - run: go install github.com/MohamedElashri/sanad/cmd/sanad@latest
      - run: sanad audit check --format json
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

See the [CI guide](docs/content/guide/ci.md) for update workflows and pull request automation.

## Development

```bash
make build
make test
make lint
make docs-build
```

The documentation site is built with Nida from `docs/`. User docs live under `docs/content/guide`, exact lookup pages under `docs/content/reference`, and contributor/internal docs under `docs/content/advanced`.


## LICENCE

This project is released under the [MIT LICENCE](./LICENSE)
