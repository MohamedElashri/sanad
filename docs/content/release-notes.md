+++
title = "Release notes"
description = "Sanad 1.0.0 · October 17, 2026"
template = "page"
+++

Sanad 1.0 manages GitHub's native Actions lockfile. GitHub enforces the pins at runtime; Sanad adds onboarding, controlled updates, policy, and review reports.

## Workflow and commands

Use `init` to establish a reviewed baseline, `sync` to reconcile workflow edits, `plan` and `update` for stable version changes, and `check` for frozen verification. Repository writes require `--write`.

Optional first-seen waiting periods cover direct and transitive changes. CI uses trusted base observations so a PR cannot establish its own dependency maturity.

## Distribution and automation

Release archives include the verified official backend for Linux, macOS, and Windows on amd64/arm64. Homebrew installs a cask, and the Nix flake installs published release archives.

The GitHub Action installs its matching immutable release and provides job summaries, annotations, and structured reports. Generated workflows support PR checks and scheduled update PRs.

## Compatibility

The commands, configuration, inline comments, and metadata from 0.x are removed. See the separate [reset guide](../guide/migration/) for older installations.

The native producer rejects actions with generated local sub-actions, including `pypa/gh-action-pypi-publish@v1.14.2`. See [Security and design](../advanced/security-model/) for the supported boundaries.

[Download v1.0.0](https://github.com/MohamedElashri/sanad/releases/tag/v1.0.0) · [Full changelog](https://github.com/MohamedElashri/sanad/blob/main/CHANGELOG.md)
