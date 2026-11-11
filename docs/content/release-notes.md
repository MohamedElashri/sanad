+++
title = "Release notes"
description = "Sanad 1.0.3 · November 11, 2026"
template = "page"
+++

Sanad 1.0 manages GitHub's native Actions lockfile. GitHub enforces the pins at runtime; Sanad adds onboarding, controlled updates, policy, and review reports.

## New in 1.0.3

This patch hardens recovery, policy enforcement, and Action installation. Recovery rejects unmanaged paths, and writes and recovery share an operating-system lock. Verification enforces release cooldowns, transitive updates obey version policy, and cached release archives are verified before fresh executables are extracted.

Workflow handling now supports YAML anchors and aliases, discovers reachable local actions in directories such as `vendor` and `dist`, and preserves case-sensitive sub-action selection. Automatic updates select only complete stable versions. Observation identities safely support Git refs containing delimiters, and existing history keeps its maturity when migrated.

When upgrading:

- Supply a trusted `--base` for online checks when either first-seen or release-cooldown policy is enabled.
- Configure `SANAD_TOKEN` for generated scheduled updates with Contents, Workflows, and Pull requests write permissions, or pass an equivalent GitHub App token. The built-in `GITHUB_TOKEN` cannot push workflow edits.
- Upgrade the CLI and Action together when they share observation history. Version 1 state remains readable; subsequent state changes write version 2.

Release cooldowns use the later of release publication and the locked commit's committer timestamp. They do not establish when a tag last moved; use first-seen policy for that waiting period.

## New in 1.0.2

Default CLI reports now show aligned terminal tables with colored headings, results, coverage, findings, and update decisions. Tables wrap to the terminal width and switch to labeled fields when full headers cannot fit. File diffs highlight additions, deletions, and hunks while preserving indentation and line breaks.

Report values stay literal, including Markdown punctuation in refs, paths, and messages. Color detection follows the output destination and respects `NO_COLOR`, dumb terminals, and redirection. Raw Markdown, JSON reports, and exit codes retain their existing behavior.

## Workflow and commands

Use `init` to establish a reviewed baseline, `sync` to reconcile workflow edits, `plan` and `update` for stable version changes, and `check` for frozen verification. Repository writes require `--write`.

Optional first-seen waiting periods cover direct and transitive changes. CI uses trusted base observations so a PR cannot establish its own dependency maturity.

## Distribution and automation

Release archives include the verified official backend for Linux, macOS, and Windows on amd64/arm64. Homebrew installs a cask, and the Nix flake installs published release archives.

The GitHub Action installs its matching immutable release and provides job summaries, annotations, and structured reports. Generated workflows support PR checks and scheduled update PRs.

## Compatibility

The commands, configuration, inline comments, and metadata from 0.x are removed. See the separate [reset guide](../guide/migration/) for older installations.

The native producer rejects actions with generated local sub-actions, including `pypa/gh-action-pypi-publish@v1.14.2`. See [Security and design](../advanced/security-model/) for the supported boundaries.

[Download v1.0.3](https://github.com/MohamedElashri/sanad/releases/tag/v1.0.3) · [Full changelog](https://github.com/MohamedElashri/sanad/blob/main/CHANGELOG.md)
