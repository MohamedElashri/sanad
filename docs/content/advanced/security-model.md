+++
title = "Security and design"
description = "Understand enforcement, trust boundaries, and what a lockfile does not cover."
weight = 10
template = "page"
+++

GitHub resolves and enforces native execution pins. Sanad adds onboarding, scoped version selection, adoption policy, and review reports around the official parser and producer.

## Who checks what

| Component | Responsibility |
| --- | --- |
| GitHub's native producer | Resolve dependencies and verify source identity, commit reachability, and reference consistency. |
| Sanad | Isolate candidate generation, check graph integrity, apply policy, and review changed dependencies and files. |
| GitHub runtime | Enforce the repository's native pins when supported workflows run. |

Repository support must be confirmed separately. A generated lockfile or successful local check does not establish runtime eligibility. See GitHub's [native lockfile documentation](https://docs.github.com/en/actions/how-tos/secure-your-work/lock-workflow-dependencies) before onboarding.

## Dependency coverage

A native lockfile records action commits, numeric repository identities, and workflow dependency coverage. Sanad evaluates changed transitive commits as well as direct actions. It also rejects dangling dependency references and incomplete transitive workflow lists locally.

Same-repository action references can be converted to `$/` so they resolve against the repository revision. Dependencies inside an external reusable workflow use the defining repository's lockfile; reports identify that boundary.

| Included in the dependency review | Outside this lockfile's guarantees |
| --- | --- |
| Direct action pins and their recorded identities. | The safety or correctness of action source code. |
| Transitive action dependencies. | Vulnerability assessment. |
| Workflow coverage and graph integrity. | Package-manager dependencies or downloaded scripts. |
| Commits changed by a proposed version update. | Docker image contents and other runtime downloads. |

## Upstream compatibility limits

The pinned producer can reject composite actions that generate local sub-actions at runtime. `pypa/gh-action-pypi-publish@v1.14.2` is one observed example. Affected repositories cannot fully onboard those workflows until the upstream limitation is resolved.

Sanad reports the failure rather than creating a fallback inline-pinning implementation. Keep the repository's existing protection until you have a supported, reviewed setup.

The native backend supports GitHub.com and GitHub Enterprise Cloud with data residency. GitHub Enterprise Server is outside its supported targets.

## Candidate isolation

Sanad generates candidates in isolated metadata snapshots and does not execute dependency code. It verifies the final candidate with a native frozen check before applying repository changes.

Selected version updates cannot move unrelated pins. Broad mutable-ref refresh is unavailable because the upstream relock operation can affect dependencies outside the selected scope.

## Waiting periods and trusted history

First-seen state is keyed by host, numeric source identity, ref, and commit. A retargeted ref starts a new clock. All newly changed transitive commits must also mature.

CI reads observation history from a trusted `--base` revision. Proposed changes cannot establish their own maturity by adding old-looking observations to the same PR. Administrators can still edit policy and history: this is reviewed Git state, not a trusted timestamp service.

Protect workflows, lockfiles, policy, and observations with appropriate review controls. See [Waiting periods](../../guide/policy/) for the operational workflow.

## Credentials and installation

Release archives include a checksum-verified official backend. The Action checks the GitHub release's immutability, validates the archive digest, and requires the CLI version to match its own release.

Authentication is host-aware. Tenant credentials are not reused for public GitHub release discovery. Environment tokens must belong to the host you intend to access; they take precedence over stored GitHub CLI credentials.

## Repository writes and recovery

Writes detect intervening edits and use a journal at `.github/sanad/transaction.json`. A multi-file write is not crash-atomic. Recovery restores the recorded state and refuses to overwrite later changes.

An unexpected journal needs investigation. Use the [recovery procedure](../troubleshooting/#interrupted-transaction) before attempting further operations.

## Pinned native components

Sanad 1.0 uses `gh-actions-lock v0.2.0` and `actions-lockfile/go v0.0.6`. Backend binaries are pinned by SHA-256. Changing either component requires the producer contract tests.

The implementation has two packages: `internal/sanad` for the core behavior and `internal/cli` for commands and presentation. Dependency resolution stays in the official producer.
