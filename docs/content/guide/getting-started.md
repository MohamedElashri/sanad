+++
title = "Getting started"
description = "Install Sanad, review your first lockfile, and verify it."
weight = 10
template = "page"
+++

Sanad manages GitHub's native Actions lockfile. Start with the CLI to create a reviewed baseline, then add the Action to verify future pull requests.

> **Check support first.** Your repository must support GitHub's [native workflow dependency lockfiles](https://docs.github.com/en/actions/how-tos/secure-your-work/lock-workflow-dependencies). A successful local check does not prove that GitHub will enforce the lockfile at runtime.

## Install Sanad

### Homebrew

```sh
brew install --cask MohamedElashri/sanad/sanad
sanad version
```

### Release archives

Download the archive for your platform from the [v1.0.0 release](https://github.com/MohamedElashri/sanad/releases/tag/v1.0.0). Extract both executables to the same directory on your `PATH`.

| Platform | Archive suffix | Executables |
| --- | --- | --- |
| Linux | `Linux_x86_64.tar.gz` or `Linux_arm64.tar.gz` | `sanad`, `gh-actions-lock` |
| macOS | `Darwin_x86_64.tar.gz` or `Darwin_arm64.tar.gz` | `sanad`, `gh-actions-lock` |
| Windows | `Windows_x86_64.zip` or `Windows_arm64.zip` | `sanad.exe`, `gh-actions-lock.exe` |

The archive includes the checksum-pinned official backend. Keep it beside Sanad so the CLI can find it without an additional download. The release also provides a SHA-256 checksum manifest.

### Nix

```sh
nix run github:MohamedElashri/sanad -- version
```

The Nix flake installs the published release and its bundled backend. It supports Linux and macOS on x86-64 and ARM64.

## Authenticate

Run commands from your repository checkout. Sanad uses `GH_TOKEN`, then `GITHUB_TOKEN`, then the credentials stored by GitHub CLI.

```sh
gh auth login
sanad doctor
```

For GitHub Enterprise Cloud with data residency, authenticate to your tenant host. Use the [repository and authentication options](../../reference/cli/#repository-and-authentication) when automatic discovery is insufficient.

## Preview the initial lockfile

```sh
sanad init --confirm-native
```

`--confirm-native` records your confirmation of GitHub support. It is not an automatic eligibility check. The command resolves the currently referenced actions and their transitive dependencies in an isolated metadata snapshot, then shows the proposed changes without writing them to your repository.

Review the chosen commits, repository identities, affected workflows, and any findings. Initial onboarding accepts these commits as the baseline; optional waiting periods apply to later changes.

## Write and verify

When the preview is correct:

```sh
sanad init --confirm-native --write
sanad check
```

Commit the generated files along with any workflow or local-action changes shown in the report.

| File | Purpose |
| --- | --- |
| `.github/workflows/actions.lock` | Native execution pins and workflow dependency coverage. |
| `.sanad.toml` | Native-support confirmation and Sanad policy. |
| `.github/sanad/state.json` | Optional first-seen observations, created when waiting-period policy needs them. |

Same-repository actions can be converted from `./path` to the native `$/path` form. Review and commit those edits too.

## Add pull-request checks

Follow [CI and automation](../ci/) to add the Sanad Action. After adding or changing a workflow, run `sanad sync --write` and commit the updated lockfile so the new workflow is covered.

For ongoing maintenance, continue with [Updating dependencies](../updating/). If setup fails, see [Troubleshooting](../../advanced/troubleshooting/).
