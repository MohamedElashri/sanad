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

Preview upgrades for all managed entries to their latest GitHub release target:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update upgrade
```

Upgrade a managed action to an explicit ref:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update upgrade --action actions/checkout --to v5
```

`upgrade` is dry-run by default. Add `--write` after reviewing the diff:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update upgrade --action actions/checkout --to v5 --write
```

You can also spell out the default all/latest-release behavior explicitly:

```bash
GITHUB_TOKEN=$(gh auth token) sanad update upgrade --all --latest-release
```

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
