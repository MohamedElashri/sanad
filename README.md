# sanad

[![CI](https://github.com/MohamedElashri/sanad/actions/workflows/ci.yml/badge.svg)](https://github.com/MohamedElashri/sanad/actions/workflows/ci.yml)
[![Pages](https://github.com/MohamedElashri/sanad/actions/workflows/pages.yml/badge.svg)](https://github.com/MohamedElashri/sanad/actions/workflows/pages.yml)
[![Release](https://github.com/MohamedElashri/sanad/actions/workflows/release.yml/badge.svg)](https://github.com/MohamedElashri/sanad/actions/workflows/release.yml)

`sanad` pins and updates GitHub Actions dependencies to immutable commit SHAs while preserving the logical refs you want to track.

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

## Quickstart

Scan workflows without making network calls:

```bash
sanad scan
```

Preview policy decisions and proposed pin updates:

```bash
GITHUB_TOKEN=$(gh auth token) sanad plan
```

Show the rewrite diff without changing files:

```bash
GITHUB_TOKEN=$(gh auth token) sanad apply --dry-run
```

Apply locally:

```bash
GITHUB_TOKEN=$(gh auth token) sanad apply --yes --write
```

Validate locally:

```bash
GITHUB_TOKEN=$(gh auth token) sanad check
```

`sanad plan`, `sanad check`, `sanad apply`, and `sanad upgrade` may contact GitHub when resolution is needed. `sanad scan` is local-only.

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

Set `[comments].write = false` to rely on `.github/sanad.lock.json` without inline `sanad` comments. See the [config reference](docs/content/reference/config.md) for exact supported keys.

Interactive apply can optionally persist branch tracking by writing `[updates].branches = "track"` after final confirmation.

## Commands

```bash
sanad scan
sanad check
sanad plan
sanad apply
sanad upgrade
sanad version
```

All commands accept:

```bash
--config .sanad.toml
--format table
--format json
```

`sanad check --format sarif` emits SARIF for code scanning, and `sanad plan --pr-body-out body.md` writes a Markdown pull request summary for automation.

`sanad upgrade --action actions/checkout --to v5` intentionally moves managed pins to a new logical ref while keeping workflow execution pinned to a full SHA.

Command-specific usage is covered in the [CLI reference](docs/content/reference/cli.md).

## GitHub Authentication

`sanad` reads tokens from environment variables:

1. `GITHUB_TOKEN`
2. `GH_TOKEN`

Tokens are used for GitHub API requests and are never printed by the CLI. Public repositories can work without a token, but authenticated requests are strongly recommended for CI and private repositories.

For local shell usage, prefer reusing the GitHub CLI token instead of pasting a token into your terminal:

```bash
GITHUB_TOKEN=$(gh auth token) sanad plan
```

Set `[github].api_url` in `.sanad.toml` when resolving refs through GitHub Enterprise.

## Security Model

The core policy is simple: workflow dependencies should run immutable full-length SHAs. Mutable tags and branches are resolved to commits, short SHAs are rejected, local and Docker actions are skipped by default, and branch or unpinned behavior must be explicitly allowed before it is managed non-interactively.

See the [security model](docs/content/advanced/security-model.md) for the full model.

## Cooldown

The default cooldown is `14d`. When an upstream tag or branch resolves to a newer SHA, `sanad` checks the candidate timestamp before adopting it. If the candidate is newer than the cooldown window, the decision is `pending-cooldown` and no rewrite is applied yet.

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
      - run: sanad check --format json
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
