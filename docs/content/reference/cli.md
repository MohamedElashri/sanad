+++
title = "CLI and policy"
description = "The complete small command surface."
template = "page"
+++

| Command | Purpose |
| --- | --- |
| `status` (or no command) | Coverage, dependencies, readiness |
| `doctor` | Tool and state diagnostics |
| `setup` | Install the exact native backend |
| `init [--automation]` | Preview initial onboarding |
| `sync` | Reconcile edits without advancing existing pins |
| `plan` / `update` | Select and explain newer versions |
| `check [--base REV]` | Online frozen verification and change policy |
| `check --offline` | Local coverage and graph integrity |
| `recover --write` | Restore an interrupted transaction |
| `config` | Effective policy as JSON |
| `version` / `completion SHELL` | Tool versions and shell completion |

`init`, `sync`, and `update` preview by default; add `--write` to apply.
Common flags: `--root`, `--repo [HOST/]OWNER/REPO`, `--config`, and
`--format table|json|markdown`. Updates accept `--action`, `--to`, `--level`, or
`--constraint`. Explicit `--to` still obeys waiting periods. Broad mutable-ref
refresh is unsupported. Branches and partial versions stay frozen until an explicit version change.

```toml
version = 1
[native]
enabled = true
[update]
level = "minor"
[observation]
minimum_age = "7d"
[actions."actions/checkout"]
constraint = ">= 6, < 8"
```

Levels: `patch`, `minor` (default), `major`. Stable releases only. Per-action
policies replace the global limit; CLI flags take precedence. Omit `observation`
for ordinary native updates. Durations accept `7d` or Go durations such as `48h`.
Dependabot release-age policy supports one `github-actions` entry with
`cooldown.default-days`; unsupported or ambiguous settings fail explicitly.

JSON reports use `version: 1` and contain coverage, verification level, findings,
dependency changes, decisions, file diffs, and whether changes were written.
Exit codes: `0` success, `1` blocking findings, `2` operational/configuration error.
Waiting candidates do not fail a plan; incoming changes violating policy fail checks.

Authentication follows the native tool: `GH_TOKEN`, `GITHUB_TOKEN`, then stored
`gh` credentials. Tenant tokens are not reused for public-host release discovery.
