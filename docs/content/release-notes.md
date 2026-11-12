+++
title = "Release notes"
description = "Sanad 1.1.0 · November 12, 2026"
template = "page"
+++

Sanad 1.1 adds reusable-workflow management, source approvals, complete action vendoring, and clearer dependency reviews to the native lockfile workflow. GitHub enforces native action pins; Sanad manages adoption, updates, policy, and review.

## New in 1.1.0

- **Reusable workflows:** `init` and `sync` pin remote calls to full commit SHAs and record associated refs and numeric identities in `.github/sanad/workflows.json`. `plan` and `update` manage tracked stable versions under adoption policy. Callee inspection checks visible direct action pins at the selected commit and leaves uninspected local implementations unconfirmed.
- **Dependency explanations:** `sanad explain` shows call paths, source identities, and stored observation dates. Update reports include commit compare links and identify new or changed sources.
- **Protection reports:** distinguish native action locks, inline workflow pins, image digests, workspace actions, runner environments, runtime tools, and uninspected shell downloads. Optional zizmor SARIF import brings existing audit findings into the report.
- **Publisher and trust policy:** protect inline SHAs in declared exported actions and require reviewed repository identities for direct and transitive sources. Trusted-base policy prevents a dependency PR from approving its own new source.
- **Complete vendoring:** `sanad vendor --action owner/repo` copies the verified source snapshot and licenses. Explicit `--to` updates keep provenance and file digests current. Ordinary updates preserve immutable snapshots, and local modifications block replacement.
- **Version selection:** explicit tag prefixes support monorepo version families; optional tag discovery supports projects with incomplete release catalogs. Shared subactions obey all selected policies without depending on call order.
- **CLI and distribution:** token-only usage, Makefile targets, and a system-language pre-commit hook keep the Action optional. Release builds produce six platform archives, SPDX inventories, and build-provenance attestations.

## Security and reliability

The bundled native producer is now `gh-actions-lock v0.4.1`. Its stricter source-reachability and graph checks reject pins accepted by older versions. Sanad also restricts recovery paths, locks transactions through validation and writes, verifies cached Action archives before extracting fresh executables, and enforces waiting periods and transitive version policy consistently.

Repository identity remains bound across action and workflow call types. Callee coverage is checked against visible action calls, vendoring verifies existing workflow pins, and canceled operations cannot start repository writes. YAML aliases, flat parallel groups, reachable local actions, and case-sensitive subaction paths are handled consistently.

## Upgrade from 1.0.x

1. Upgrade the CLI and Action together to 1.1.0. Configuration version 1, report version 1, and existing command names remain supported.
2. Run `sanad sync` to inspect reusable-workflow onboarding, then `sanad sync --write`. Commit the reported workflow edits and `.github/sanad/workflows.json`. Waiting periods can produce an observation-only change before adoption; merge that record through the usual review process and synchronize again when mature.
3. Use a trusted `--base` for online checks when first-seen, release-cooldown, or source-approval policy applies. The explicit developer command `check --offline --coverage-only` checks local structure without enforcing adoption policy.
4. Provide `SANAD_TOKEN` for generated update automation, with Contents, Workflows, and Pull requests write permissions, or pass an equivalent GitHub App token. The built-in `GITHUB_TOKEN` is rejected for updates.

Observation state version 1 remains readable and is written as version 2 on its next change without resetting established dates. Review stricter producer findings and reconcile the referenced sources rather than manually changing pins to bypass them.

Release cooldowns use the later of publication and the locked commit's committer timestamp. They do not establish when a tag moved; use first-seen policy when that distinction matters. Tag-only automatic candidates with unknown publication age cannot satisfy a release cooldown.

See [updating dependencies](../guide/updating/), [policy](../guide/policy/), and [CLI-only integration](../guide/cli-only/) for the new workflows.

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

[Download v1.1.0](https://github.com/MohamedElashri/sanad/releases/tag/v1.1.0) · [Full changelog](https://github.com/MohamedElashri/sanad/blob/main/CHANGELOG.md)
