+++
title = "Getting Started"
description = "Install Sanad, inspect a repository, and pin your first workflow actions."
weight = 10
template = "page"
+++

Sanad is a normal CLI. It does not need a GitHub Action wrapper.

## Install with Homebrew

On macOS or Linux with Homebrew:

```bash
brew tap MohamedElashri/sanad
brew install sanad
sanad version
```

Homebrew installs the formula from the `MohamedElashri/homebrew-sanad` tap. Release automation updates that formula from the published archives and checksums.

## Install with Nix

Run the packaged release directly:

```bash
nix run github:MohamedElashri/sanad -- version
```

Or install it into your profile:

```bash
nix profile install github:MohamedElashri/sanad
sanad version
```

The flake uses the published release archives for Linux and macOS on `x86_64` and `aarch64`, with fixed hashes derived from the release checksums.

## Install from source

Install the latest tagged release with Go:

```bash
go install github.com/MohamedElashri/sanad/cmd/sanad@latest
```

## Manual prebuilt archive install

Tagged releases also publish Linux, macOS, and Windows archives on [GitHub Releases](https://github.com/MohamedElashri/sanad/releases). Download the archive for your platform and verify it against the published checksums file.

Check the installed binary:

```bash
sanad version
```

## Scan workflows

Run a local scan first. This does not contact GitHub:

```bash
sanad scan
```

Use JSON when another tool will consume the result:

```bash
sanad --format json scan
```

## Preview changes

Planning resolves GitHub refs and applies policy without changing files:

```bash
GITHUB_TOKEN=$(gh auth token) sanad plan
```

The table output summarizes what Sanad found and whether each action is unchanged, eligible for update, pending cooldown, skipped, or blocked by policy.

## Apply changes

Preview the exact rewrite first:

```bash
GITHUB_TOKEN=$(gh auth token) sanad apply --dry-run
```

Then write the workflow changes and lockfile:

```bash
GITHUB_TOKEN=$(gh auth token) sanad apply --yes --write
```

Sanad rewrites only the relevant scalar values. It does not serialize or reformat the whole YAML document.

## Check policy

Use `check` when a repository should already comply:

```bash
GITHUB_TOKEN=$(gh auth token) sanad check
```

Exit code `0` means the check passed. Exit code `1` means policy violations or required changes were found.
