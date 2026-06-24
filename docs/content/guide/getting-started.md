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
brew tap MohamedElashri/sanad && brew install sanad
```
Check that it is installed 

```bash
sanad version
```

Homebrew installs the formula from the `MohamedElashri/homebrew-sanad` tap. Release automation updates that formula from the published archives and checksums.

Homebrew installs shell completions for bash, zsh, fish, and PowerShell automatically.

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

The Nix package installs bash, zsh, and fish completions automatically.

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

## Scan workflows

Run a local scan first. This does not contact GitHub:

```bash
sanad audit scan
```

Use JSON when another tool will consume the result:

```bash
sanad --format json audit scan
```

## Preview changes

Planning resolves GitHub refs and applies policy without changing files:

```bash
GITHUB_TOKEN=$(gh auth token) sanad audit plan
```

The table output summarizes what Sanad found and whether each action is unchanged, eligible for update, pending cooldown, skipped, or blocked by policy.

## Apply changes

Preview the proposed updates first:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update apply --dry-run
```

Add `--diff` to inspect the exact rewrite:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update apply --dry-run --diff
```

Then write the workflow changes and lockfile:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update apply --yes --write
```

Sanad rewrites only the relevant scalar values. It does not serialize or reformat the whole YAML document.

## Check policy

Use `check` when a repository should already comply:

```bash
GITHUB_TOKEN=$(gh auth token) sanad audit check
```

Exit code `0` means the check passed. Exit code `1` means policy violations or required changes were found.
