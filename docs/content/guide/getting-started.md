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

## Initialize Sanad

The easiest way to initialize sanad and pin your workflows is to run:

```bash
GITHUB_TOKEN=$(gh auth token) sanad start
```

This command will:
1. Create a default `.sanad.toml` config file if one doesn't exist.
2. Scan your workflows and securely resolve all action references.
3. Apply the immutable SHAs to your workflows.
4. Create the lockfile at `.github/sanad.lock.json`.

If you prefer to preview changes without applying them, you can use:

```bash
GITHUB_TOKEN=$(gh auth token) sanad plan
```

Add `--diff` to inspect the exact rewrite:

```bash
GITHUB_TOKEN=$(gh auth token) sanad apply --dry-run --diff
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
