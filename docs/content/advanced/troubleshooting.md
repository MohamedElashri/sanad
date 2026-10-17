+++
title = "Troubleshooting"
description = "Diagnose setup, verification, policy, and interrupted writes."
weight = 20
template = "page"
+++

Start with the repository diagnostics and an online check:

```sh
sanad doctor
sanad check
```

Keep the finding code and full report when investigating a failure. `sanad check --format json` provides structured evidence for automation or bug reports.

## Native support is unknown

Sanad requires an explicit confirmation before it writes repository files. Verify GitHub eligibility and runtime enforcement, then initialize:

```sh
sanad init --confirm-native
sanad init --confirm-native --write
```

Do not use the flag merely to silence a finding. Local generation and offline coverage do not prove that GitHub enforces the lockfile.

## Backend is missing or has a checksum mismatch

```sh
sanad setup
```

Setup installs the exact official backend. If you supplied `SANAD_BACKEND`, check that it points to the pinned binary for your platform. A custom producer or modified binary is rejected.

Release archives contain both executables; keep them together. Offline checks need the backend to be installed before network access is removed.

## Authentication or repository discovery fails

Check that the repository remote identifies the intended GitHub host and repository. If necessary, specify it explicitly:

```sh
sanad check --repo owner/repository
```

For a data-residency tenant:

```sh
gh auth login --hostname example.ghe.com
sanad check --repo example.ghe.com/owner/repository
```

`GH_TOKEN` and `GITHUB_TOKEN` override stored credentials. Check which host those environment variables are intended for without printing or sharing their values.

## A workflow is uncovered or a dependency is missing

After changing a workflow, reconcile the graph and review the diff:

```sh
sanad sync
sanad sync --write
sanad check
```

Commit the workflow and the updated lockfile together. A malformed lockfile with dangling graph references can fail before the backend runs. Restore a reviewed valid version, then regenerate changes; do not invent dependency IDs or commits by hand.

## A locked ref or repository identity changed

Findings such as `repo-replaced`, `source-identity-changed`, or `unreachable-pin` need investigation. Verify the upstream repository and the intended code before changing the dependency.

Sanad's scoped operations do not expose a broad accept-moved or relock shortcut. Restore a trusted lockfile or choose an explicitly reviewed target after investigating the finding.

## Updates remain pending

Read the decision reason and `eligible_at` timestamp. With first-seen policy, run `sanad update --write` to record observations and merge the state change into the trusted base branch.

Temporary caches and unmerged observation-only PRs do not establish trusted history. A newly introduced transitive dependency can start another waiting period. See [Waiting periods](../../guide/policy/).

## CI requires a trusted base

Check out enough history and pass the pull request's base SHA:

```yaml
- uses: actions/checkout@v7.0.1
  with:
    fetch-depth: 0
- uses: MohamedElashri/sanad@v1.0.0
  with:
    mode: check
    base: ${{ github.event.pull_request.base.sha }}
```

A local online check uses `--base origin/main`. Offline verification cannot enforce adoption policy.

## Interrupted transaction

If `.github/sanad/transaction.json` exists, stop editing managed files and inspect the working tree. Then run:

```sh
sanad recover --write
sanad status
```

Recovery restores the recorded files. If files were edited after the interrupted operation, recovery refuses to overwrite them. Preserve those changes and investigate the journal before making manual repairs.

## Generated workflows already exist

`init --automation` refuses to replace an independently maintained workflow with different content. Compare the generated intent with the existing workflow, then maintain the file yourself or move it aside deliberately before generating a replacement.

## The producer rejects a generated local sub-action

This is an upstream compatibility limitation, including the observed `pypa/gh-action-pypi-publish@v1.14.2` case. Retain the existing protection for the affected workflow and follow the upstream tool's resolution. See [Security and design](../security-model/#upstream-compatibility-limits).

## Older configuration is rejected

Sanad 1.0 has no compatibility mode for legacy metadata. Use the separate [reset guide](../../guide/migration/) to remove the old state and establish a reviewed native baseline.
