# Changelog

All notable changes to Sanad are documented here.

## 0.1.2 - 2026-05-19

### Security

- Stopped sending `GITHUB_TOKEN` or `GH_TOKEN` to custom `[github].api_url` endpoints by default. GitHub Enterprise users can opt in explicitly with `[github].send_token_to_custom_api_url = true`.
- Hardened `[github].api_url` validation so custom GitHub API endpoints must use HTTPS and cannot include embedded credentials.

### Fixed

- Detected lockfile entries that no longer match the workflow action at the same YAML node, reporting a metadata conflict instead of applying stale tracking metadata to a different action.
- Made `sanad plan`, `sanad apply`, and `sanad upgrade` handle lockfile and inline-comment metadata recovery consistently.

## 0.1.1 - 2026-05-18

### Added

- Added full TOML parsing through `github.com/BurntSushi/toml`, including richer nested config support and overlay behavior.
- Added configurable metadata comments through `[comments].write` and validated comment format handling.
- Added fail-closed `[security]` config validation for strict full-SHA and source-repository behavior.
- Added unpinned action discovery modes for `updates.unpinned = "default-branch"` and `updates.unpinned = "latest-release"`.
- Added `sanad upgrade` for intentionally moving managed pins from one logical ref to another.
- Added optional interactive persistence for branch tracking choices.
- Added a Nida-powered documentation site and generated release notes from this changelog.
- Added Dependabot configuration and pinned the repository's own workflow actions with Sanad metadata.
- Added Homebrew tap publishing through GoReleaser for the external `homebrew-sanad` tap.
- Added a Nix flake that installs verified Sanad release archives on Linux and macOS.


## v0.1.0 - 2026-05-18

First public release.

### Added

- Added `sanad scan`, `plan`, `check`, `apply`, and `version`.
- Added GitHub Actions `uses:` discovery for workflow files, reusable workflows, local actions, Docker actions, invalid references, short SHAs, and full SHA pins.
- Added GitHub ref resolution for tags, branches, and full SHAs.
- Added cooldown-aware update planning so newly moved refs are not adopted immediately.
- Added byte-preserving workflow rewrites with inline `sanad: ref=...` metadata.
- Added `.github/sanad.lock.json` metadata support for deterministic tracking.
- Added JSON, table, SARIF, unified diff, and pull request body outputs.
- Added interactive apply flows for unpinned actions, unmanaged full-SHA pins, and denied branch refs.
- Added update policies, ignore rules, organization policy files, GitHub API endpoint configuration, and strict default security behavior.
- Added GoReleaser packaging for Linux, macOS, and Windows archives with checksums.

### Notes

- `v0.1.0` is intentionally focused on GitHub Actions workflow dependencies. It is not a general dependency updater or vulnerability scanner.
