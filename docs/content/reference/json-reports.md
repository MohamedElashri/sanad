+++
title = "JSON Reports"
description = "Versioned machine-readable contracts for checks, plans, applies, and upgrades."
weight = 30
template = "page"
+++

Sanad's automation commands emit versioned JSON reports. Consumers must inspect the top-level `version` before reading the remaining fields.

| Command | Current version | Schema |
| --- | ---: | --- |
| `check --format json` | 1 | [`check-v1.schema.json`](../../schema/check-v1.schema.json) |
| `plan --format json` | 1 | [`plan-v1.schema.json`](../../schema/plan-v1.schema.json) |
| `apply --format json` | 1 | [`plan-v1.schema.json`](../../schema/plan-v1.schema.json) |
| `upgrade --format json` | 2 | [`upgrade-v2.schema.json`](../../schema/upgrade-v2.schema.json) |

`apply` uses the plan report because it evaluates the same workflow decisions before optionally writing them. Whether files actually changed is execution state rather than part of the plan contract; integrations should compare the managed files before and after a write.

## Compatibility policy

Within one report version, Sanad may:

- add a new decision or reason-code string;
- populate an optional field that was previously absent;
- improve human-readable `reason` text.

Sanad will increment the report version before it removes a field, renames a field, changes a field's JSON type, changes the meaning of a field, or makes an optional field required.

Consumers should treat unknown decision and reason-code values as values they do not yet understand, rather than as successful decisions. Empty report collections are encoded as `[]`, not `null`.

The command's process exit code remains authoritative. In particular, `check` writes its complete JSON report before exiting with code `1` for policy violations. Other exit codes identify configuration, resolution, API, rate-limit, rewrite, filesystem, and internal failures as documented in the [CLI reference](cli.md#exit-codes).

## Check report

The check report contains `passed`, aggregate counts under `summary`, and source-located `violations`. Line and column numbers are one-based and can be used for CI annotations.

## Plan and apply report

The plan report groups action decisions by workflow file. Its summary separates available updates, cooldown-pending candidates, policy violations, already-pinned references, and skipped entries.

## Upgrade report

The upgrade report lists matched managed actions and their automatic or explicit release selection decisions. Version 2 includes the considered release `candidates`, allowing automation to explain why an older eligible release was selected while a newer release was still cooling down.
