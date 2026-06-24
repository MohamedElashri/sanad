+++
title = "Updating Actions"
description = "Use apply for tracked SHA updates and upgrade for intentional logical-ref changes."
weight = 30
template = "page"
+++

Sanad separates two workflows that are easy to blur together.

`sanad update apply` keeps tracking the current logical ref. If a workflow says a pinned action tracks `v4`, apply checks where `v4` points now and updates only the pinned SHA when policy allows it.

`sanad update upgrade` intentionally changes the logical ref itself. Use it when you want to move `actions/checkout` from `v4` to `v5`.

## Refresh existing pins

Preview tracked updates:

```bash
GITHUB_TOKEN=$(gh auth token) sanad audit plan
```

Apply eligible updates:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update apply --yes --write
```

By default, Sanad evaluates cooldown with upstream release, tag, or commit timestamps. If `cooldown_source = "first-seen"` is configured, Sanad instead records when a new candidate SHA was first seen locally. If the candidate has not satisfied the configured cooldown, Sanad reports `pending-cooldown` and does not rewrite that entry yet.

## Change the logical ref

Preview the highest stable release allowed by the configured SemVer policy and cooldown:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update upgrade
```

Restrict one run to minor upgrades, or use an absolute constraint:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update upgrade --all --level minor
GITHUB_TOKEN=$(gh auth token) sanad update upgrade --all --constraint '< 6'
```

The default `--selection latest-eligible` skips newer releases that are still cooling down and selects the highest mature match. Use `--selection latest` when the command should wait for the newest matching release instead.

Upgrade a managed action to an explicit ref:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update upgrade --action actions/checkout --to v5
```

`upgrade` is dry-run by default and shows the decision table without a file patch. Add `--diff` to inspect the patch:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update upgrade --action actions/checkout --to v5 --diff
```

Add `--write` to apply the upgrade:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update upgrade --action actions/checkout --to v5 --write
```

The legacy latest-release spelling selects only the newest matching release:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update upgrade --all --latest-release
```

With `cooldown_source = "first-seen"`, use `--write` to record observations even when no workflow rewrite is eligible. The lockfile keeps multiple observed candidates so an older mature release can be selected while a newer release starts its own cooldown.

`sanad update upgrade` only operates on managed full-SHA pins. It does not convert unmanaged pins or unpinned mutable refs; use `sanad update apply` or interactive apply for those.

## Recover a stale lockfile

Dependabot or a manual workflow edit can update a pinned SHA while `.github/sanad.lock.json` still records the previous pin. Sanad reconciles safe drift from current workflow content, so start by inspecting the lockfile instead of deleting it:

```bash
sanad lock status
```

Preview the repair:

```bash
sanad lock repair --dry-run
```

Apply safe fixes when the diagnostics are repairable:

```bash
sanad lock repair --write
```

Use `sanad lock refresh --write` when you want to rebuild all active managed lock entries from current workflows. Use `sanad lock prune --write` when you only want to remove entries for deleted workflow nodes.

Blocking diagnostics such as malformed JSON, duplicate entries, invalid SHA fields, invalid timestamps, or invalid inline comments need explicit manual correction before Sanad writes the lockfile.
