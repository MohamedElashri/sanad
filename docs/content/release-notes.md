+++
title = "Release notes"
description = "Sanad 1.1.0 · November 12, 2026"
template = "page"
+++

## Features

- Pin and update reusable workflows.
- Explain dependency paths and source identities.
- Report coverage gaps and import zizmor findings.
- Protect published pins and approve trusted sources.
- Vendor verified action snapshots.
- Support tag families and tag-based updates.
- Add CLI hooks, SBOMs, and build provenance.

## Fixes

- Upgrade the native backend to v0.4.1.
- Harden recovery, locking, and cached binaries.
- Enforce cooldowns and transitive update policies.
- Fix YAML aliases, parallel groups, and subaction policies.
- Preserve identities, vendor snapshots, and observation history.
- Correct callee coverage and stop canceled writes.

## Upgrade

Upgrade the CLI and Action together, then run `sanad sync --write`. Adoption
checks need a trusted `--base`. Scheduled updates need a workflow-capable
`SANAD_TOKEN`.

[Upgrade guide](../guide/updating/) ·
[Download v1.1.0](https://github.com/MohamedElashri/sanad/releases/tag/v1.1.0) ·
[Full changelog](https://github.com/MohamedElashri/sanad/blob/main/CHANGELOG.md)
