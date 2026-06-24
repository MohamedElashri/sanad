# Changelog

All notable changes to Sanad are documented here.

## 0.1.5 - 2026-06-24

### Added

- Added SemVer upgrade levels, arbitrary constraints, per-action overrides, and `latest` versus `latest-eligible` release selection.
- Added paginated stable GitHub release discovery and audit details for every newer candidate considered by `sanad update upgrade`.

### Changed

- Made bare `sanad update upgrade` choose the highest cooldown-eligible stable release instead of always waiting for GitHub's single latest release.
- Upgraded the lockfile to version 2 with multi-candidate first-seen history and backward migration from version 1.

### Fixed

- Prevented audit commands, scoped update operations, and `lock repair` from dropping unrelated lockfile entries. Entry deletion now requires explicit `lock refresh` or `lock prune --write` intent.

## 0.1.4 - 2026-06-14

### Added

- Added shell completion generation and user-level completion installation for bash, zsh, fish, and PowerShell.
- Added `sanad audit`, `sanad update`, and `sanad lock` command groups. The canonical workflow commands are now `sanad audit scan`, `sanad audit plan`, `sanad audit check`, `sanad update apply`, and `sanad update upgrade`.
- Added `sanad lock status`, `sanad lock refresh`, `sanad lock repair`, and `sanad lock prune` for inspecting and safely repairing stale `.github/sanad.lock.json` entries after Dependabot or manual workflow edits.

### Changed

- Kept `sanad scan`, `sanad plan`, `sanad check`, `sanad apply`, and `sanad upgrade` as hidden compatibility aliases for the migration period. New docs and completion output prefer the nested commands.
- Reconciled safe lockfile drift from current workflow content and inline `sanad` comments instead of requiring users to delete the lockfile when a valid stale entry can be repaired.

## 0.1.3 - 2026-05-19

### Security

- Removed repository-configurable custom GitHub API endpoints. Sanad now resolves refs through `github.com` only, so repository config cannot redirect CI tokens to another host.
- Added lockfile pin-drift detection so a managed workflow SHA that no longer matches `.github/sanad.lock.json` is reported as invalid metadata instead of being treated as a normal managed update.
- Restricted configured `workflow_paths` to relative in-repository paths and rejected workflow YAML symlinks during discovery.
- Made `sanad check` fail by default when a managed pin has an eligible update or cooldown-pending candidate. Use `--allow-pending-cooldown` only when pending managed candidates should be allowed in CI.

### Added

- Added `cooldown_source` with the default `source` mode for upstream release/tag/commit timestamps and an optional stricter `first-seen` mode based on `.github/sanad.lock.json` candidate observation time.
- Added `candidate_sha` and `candidate_seen_at` lockfile fields to record pending managed candidates for auditability and first-seen cooldown mode.
- Added colorized table output with CLI and environment controls for color behavior.

### Changed

- Made bare `sanad upgrade` a safe dry run by default, using all managed pins and latest GitHub releases unless explicit selector or target flags are passed.
- Add colors to cli output to make better UX.

### Fixed

- Improved command error reporting so CLI errors are printed consistently.

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
