+++
title = "Release Workflow"
description = "How Sanad release artifacts and docs are published."
weight = 40
template = "page"
+++

Sanad uses GoReleaser for tagged releases.

Release publishing is triggered by tags that match `v*`:

```bash
git tag v0.1.0
git push origin v0.1.0
```

GoReleaser builds Linux, macOS, and Windows archives for amd64 and arm64, includes README, LICENSE, and `.sanad.toml.example`, and publishes SHA-256 checksums.

## Homebrew publishing

Tagged releases also update the external Homebrew tap at `MohamedElashri/homebrew-sanad`.

The tap keeps package metadata outside the main source repository, matching the Nida release pattern. GoReleaser writes `Formula/sanad.rb` with platform-specific release archive URLs and SHA-256 values from the release artifacts.

The release workflow needs a repository secret named `HOMEBREW_TAP_GITHUB_TOKEN`. It must be a token with contents write access to `MohamedElashri/homebrew-sanad`; the default `GITHUB_TOKEN` can publish the Sanad release, but cannot write to a separate tap repository.

Users install through:

```bash
brew tap MohamedElashri/sanad &&  brew install sanad
```

## Nix flake

The repository includes `flake.nix` for Linux and macOS on `x86_64` and `aarch64`.

The flake packages the published release archives instead of rebuilding from source. This keeps the package aligned with the release checksums and avoids a separate Nix vendor hash for Go modules.

```bash
nix run github:MohamedElashri/sanad -- version
nix profile install github:MohamedElashri/sanad
```

When cutting a new release, update the flake version, artifact names if needed, and fixed hashes from the new `sanad_<version>_checksums.txt` file.

The `version` command is populated through linker metadata in release builds:

```bash
sanad version
```

## Documentation publishing

The docs site is built with Nida from `docs/` and deployed with GitHub Pages.

The Pages workflow:

1. Checks out the repository.
2. Sets up Go.
3. Installs a pinned Nida release.
4. Generates `docs/content/release-notes.md` from `CHANGELOG.md`.
5. Builds `docs/public`.
6. Uploads and deploys the Pages artifact.

Generated docs output is not committed.
