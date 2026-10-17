+++
title = "Configuration"
description = "Version limits, waiting periods, and native-support confirmation."
weight = 20
template = "page"
+++

Sanad reads `.sanad.toml` from the repository root by default. Use `--config` for another repository-relative path. Configuration is strict: unknown keys, invalid values, and incompatible policy options produce an error.

## Minimal configuration

```toml
version = 1

[native]
enabled = true

[update]
level = "minor"
```

`sanad init --confirm-native --write` writes the native-support confirmation. Set `native.enabled` only after verifying that GitHub enforces native lockfiles for your repository. It is an explicit confirmation, not an entitlement detector.

Without a policy file, read-only commands use defaults. Repository writes require native support to be confirmed.

## Configuration fields

| Setting | Default | Meaning |
| --- | --- | --- |
| `version` | Required in a file | Configuration schema version; must be `1`. |
| `native.enabled` | `false` | Your confirmation of repository support. |
| `update.level` | `minor` when no global range is set | Maximum automatic update level. |
| `update.constraint` | Unset | SemVer range instead of a level. |
| `observation.minimum_age` | Unset | Minimum first-seen age for changed commits. |
| `actions."OWNER/REPO"` | Unset | Per-action version policy. |

Inspect the effective configuration:

```sh
sanad config
sanad config --config .github/sanad.toml
```

## Version update levels

| Level | Automatic candidates |
| --- | --- |
| `patch` | Newer stable releases within the same major and minor version. |
| `minor` | Newer stable releases within the same major version. |
| `major` | Any newer stable release. |

Only complete stable versions update automatically. Branches, partial versions such as `v7`, and prerelease refs remain frozen until you choose an explicit target.

## SemVer constraints

Use a constraint when a simple level is not sufficient. A global constraint applies to every action; use a per-action override when their version ranges differ:

```toml
[update]
constraint = ">= 6, < 8"
```

Do not set `level` and `constraint` in the same policy. Constraints filter stable version candidates; they do not replace native verification or adoption waiting periods.

## Per-action overrides

```toml
version = 1

[native]
enabled = true

[update]
level = "minor"

[actions."actions/checkout"]
constraint = ">= 6, < 8"

[actions."actions/setup-node"]
level = "patch"
```

A per-action policy replaces the global version limit for that action. CLI `--level` or `--constraint` selection takes precedence. Adoption waiting periods remain repository-wide and apply to the resulting changed dependency graph.

## First-seen waiting period

```toml
[observation]
minimum_age = "7d"
```

Accepted values include `7d`, `48h`, and `0h`. Negative and invalid durations are rejected. Observations are stored in `.github/sanad/state.json`; commit them to the trusted base branch. See [Waiting periods](../../guide/policy/) before enabling this policy in CI.

## Dependabot release-age policy

Sanad supports one repository-wide `github-actions` entry with `cooldown.default-days`:

```yaml
version: 2
updates:
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
    cooldown:
      default-days: 7
```

The minimum age applies to changed direct and transitive commits. Other cooldown keys and multiple `github-actions` entries are rejected instead of being interpreted ambiguously. When both release-age and first-seen policy are enabled, both must pass.

## Files to commit

Commit `.sanad.toml`, `.github/workflows/actions.lock`, and any reported workflow or local-action edits. Commit `.github/sanad/state.json` when observation policy creates or updates it.

Do not treat temporary caches as observation history. The transaction journal is recovery metadata; an unexpected `.github/sanad/transaction.json` needs investigation and recovery rather than a normal commit.
