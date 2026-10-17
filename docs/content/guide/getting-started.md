+++
title = "Getting started"
description = "Install Sanad and create a native lockfile."
template = "page"
+++

Install with Homebrew:

```sh
brew install --cask MohamedElashri/sanad/sanad
```

Or download a [v1.0.0 archive](https://github.com/MohamedElashri/sanad/releases/tag/v1.0.0)
for Linux, macOS, or Windows on amd64/arm64. Extract both `sanad` and
`gh-actions-lock` to the same directory on your `PATH`.

[Verify GitHub eligibility](https://docs.github.com/en/actions/how-tos/secure-your-work/lock-workflow-dependencies)
for native lockfile enforcement. In your repository:

```sh
gh auth login
sanad init --confirm-native
sanad init --confirm-native --write
sanad check
```

Authentication also accepts `GH_TOKEN` or `GITHUB_TOKEN`. `--confirm-native`
records your confirmation of GitHub support; it does not detect entitlement.
Review the initial dependency graph and commits before writing the baseline.
Commit `.github/workflows/actions.lock` and `.sanad.toml`.

```sh
sanad sync --write           # reconcile workflow edits, preserving existing pins
sanad plan                  # inspect eligible updates
sanad update --write         # apply them
```

Commands preview changes unless you pass `--write`. Add [CI verification](../ci/)
to check pull requests. Configure version limits or waiting periods in the
[policy reference](../../reference/cli/).

Source installations fetch the checksum-pinned official backend on first online
use. `sanad setup` installs it ahead of time. `SANAD_BACKEND` can point to the exact
official binary. `sanad check --offline` checks local coverage and graph integrity;
it does not verify source identity or commit reachability.
