+++
title = "Updating Actions"
description = "Use apply for tracked SHA updates and upgrade for intentional logical-ref changes."
weight = 30
template = "page"
+++

Sanad separates two workflows that are easy to blur together.

`sanad apply` keeps tracking the current logical ref. If a workflow says a pinned action tracks `v4`, apply checks where `v4` points now and updates only the pinned SHA when policy allows it.

`sanad upgrade` intentionally changes the logical ref itself. Use it when you want to move `actions/checkout` from `v4` to `v5`.

## Refresh existing pins

Preview tracked updates:

```bash
GITHUB_TOKEN=$(gh auth token) sanad plan
```

Apply eligible updates:

```bash
GITHUB_TOKEN=$(gh auth token) sanad apply --yes --write
```

If a candidate commit is newer than the configured cooldown, Sanad reports `pending-cooldown` and does not rewrite that entry yet.

## Change the logical ref

Preview upgrades for all managed entries to their latest GitHub release target:

```bash
GITHUB_TOKEN=$(gh auth token) sanad upgrade
```

Upgrade a managed action to an explicit ref:

```bash
GITHUB_TOKEN=$(gh auth token) sanad upgrade --action actions/checkout --to v5
```

`upgrade` is dry-run by default. Add `--write` after reviewing the diff:

```bash
GITHUB_TOKEN=$(gh auth token) sanad upgrade --action actions/checkout --to v5 --write
```

You can also spell out the default all/latest-release behavior explicitly:

```bash
GITHUB_TOKEN=$(gh auth token) sanad upgrade --all --latest-release
```

`upgrade` only operates on managed full-SHA pins. It does not convert unmanaged pins or unpinned mutable refs; use `apply` or interactive apply for those.
