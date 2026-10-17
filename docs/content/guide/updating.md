+++
title = "Updating dependencies"
description = "Reconcile workflow edits and apply reviewed version changes."
weight = 20
template = "page"
+++

Use `sync` when workflow contents change and `update` when you want newer dependency versions. Both preview their proposed repository changes unless you add `--write`.

## Check the current state

```sh
sanad status
sanad check
```

`status` shows recorded dependencies and workflow coverage. `check` performs online frozen verification, including source identity and commit reachability. Status alone is not a verification result.

## Reconcile a workflow edit

After adding an action, removing a workflow, or changing a readable `uses:` reference:

```sh
sanad sync
sanad sync --write
sanad check
```

Synchronization adds or removes dependency coverage while preserving existing pins. It does not broadly advance mutable tags or branches. Review every reported file and commit the workflow edits together with their updated lockfile.

If adoption policy delays a new dependency, follow the report's decision and eligible date. Do not edit execution pins to bypass the waiting period.

## Review available updates

```sh
sanad plan
```

By default, Sanad considers newer stable releases within the current major version. It examines candidates from newest to oldest and selects the highest version that satisfies policy. A newer pending release does not prevent an older eligible release from being selected.

| Decision | Meaning | Next action |
| --- | --- | --- |
| `eligible` | The version and every changed dependency satisfy policy. | Review and apply the update. |
| `pending` | A release-age or first-seen waiting period has not elapsed. | Record observations if needed, then check the eligible date. |
| `unchanged` | No matching newer release is available, or the requested ref is already tracked. | Keep the current lockfile. |
| `frozen` | The current ref is not a complete stable version. | Choose an explicit target if you want to change it. |

The review includes transitive changes and affected workflows. A version change can introduce more than one new commit.

## Apply eligible versions

```sh
sanad update
sanad update --write
sanad check
```

The first command is a preview. The second writes eligible changes and, when configured, candidate observations. Commit all reported files; an observation-only change is still useful even when no version advances.

> **Review before merging.** A verified lockfile establishes source identity and pinned commits. It does not establish that the action's code is safe or free of vulnerabilities.

## Select an action or version range

Limit the operation to one action:

```sh
sanad plan --action actions/checkout
sanad update --action actions/checkout --write
```

Change the permitted update level or use a SemVer constraint:

```sh
sanad plan --level patch
sanad plan --level major
sanad plan --constraint '>= 6, < 8'
```

`--level` and `--constraint` are mutually exclusive. See [Configuration](../../reference/configuration/) for repository-wide and per-action policies.

## Choose an explicit target

```sh
sanad update --action actions/checkout --to v7.0.1
sanad update --action actions/checkout --to v7.0.1 --write
```

An explicit target still obeys adoption waiting periods. Branches, partial versions such as `v7`, and prerelease refs remain frozen during automatic version selection. Broad mutable-ref relocking is intentionally unavailable because it can move unrelated pins.

## Verify a proposed change against its base

```sh
sanad check --base origin/main
```

Use a fetched, trusted base revision when evaluating changes with adoption policy. Observations added only by a pull request cannot make that request's dependencies mature. See [Waiting periods](../policy/) for the full observation workflow.
