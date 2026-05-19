+++
title = "Security Model"
description = "What Sanad protects, what it verifies, and what remains out of scope."
weight = 10
template = "page"
+++

Sanad's core rule is simple: GitHub Actions workflow dependencies should run immutable full-length commit SHAs.

Mutable refs are still useful as update channels, so Sanad preserves the logical ref in metadata:

```yaml
- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # sanad: ref=v4
```

The workflow runs the SHA. Sanad resolves `v4` on future runs to decide whether the pin should move.

## Protected cases

Sanad rejects short SHAs because they are ambiguous and weaker than full commit IDs.

Sanad rejects branch refs by default because branches usually move more freely than release tags.

Sanad rejects unpinned `owner/repo` refs by default. Repositories can explicitly configure default-branch or latest-release discovery.

Sanad verifies resolved SHAs through the declared source repository. Relaxing that behavior is rejected by config validation.

## Cooldown

Cooldown delays adoption of newly resolved commits. By default, Sanad uses upstream release, tag, or commit timestamps as the adoption clock. This keeps normal action updates ergonomic while still refusing missing or future timestamps.

For stricter repositories, set:

```toml
cooldown_source = "first-seen"
```

In that mode, Sanad uses the time a candidate SHA was first recorded in `.github/sanad.lock.json`.

The default cooldown is:

```toml
cooldown = "14d"
```

If a candidate has not satisfied the selected cooldown source, Sanad reports `pending-cooldown` and does not rewrite it.

## Non-goals

Sanad is not a vulnerability scanner, a general dependency updater, a YAML linter, or a GitHub Actions runner.

Sanad does not update Docker image tags and does not rewrite local actions.
