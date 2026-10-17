+++
title = "CLI commands"
description = "Commands, flags, output formats, and exit codes."
weight = 10
template = "page"
+++

Run Sanad from a repository checkout, or pass `--root` to select one. Running `sanad` without a subcommand is equivalent to `sanad status`.

## Command overview

| Command | Purpose | Writes repository files? |
| --- | --- | --- |
| `status` | Show recorded coverage, dependencies, and readiness. | No. |
| `doctor` | Diagnose repository state, backend, and authentication. | No. |
| `setup` | Install and verify the exact official backend. | No repository edits; may populate the tool cache. |
| `init` | Preview native onboarding and establish an initial baseline. | Only with `--write`. |
| `sync` | Reconcile workflow edits without advancing existing pins. | Only with `--write`. |
| `plan` | Explain eligible stable version updates. | No. |
| `update` | Apply eligible updates and record observations. | Only with `--write`. |
| `check` | Run frozen verification and optional base comparison. | No. |
| `recover` | Restore files from an interrupted transaction. | Only with `--write`. |
| `config` | Print the effective configuration as JSON. | No. |
| `version` | Print Sanad and native component versions. | No. |
| `completion SHELL` | Generate shell completion. | Writes to standard output. |

## Common flags

| Flag | Default | Description |
| --- | --- | --- |
| `--root PATH` | Discovered repository root | Select the repository directory. |
| `--repo [HOST/]OWNER/REPO` | Discovered repository identity | Select the GitHub host and repository. |
| `--config PATH` | `.sanad.toml` | Select a repository-relative policy file. |
| `--format FORMAT` | `table` | `table`, `json`, or `markdown` for report-producing commands. |

`table` shows the readable review and proposed file diffs. `markdown` produces a review summary. JSON includes the full structured report and proposed file diffs. `config` always prints JSON.

Configuration paths must remain inside the repository. Commands do not change the invoking process's working directory.

## Initialization and synchronization

```sh
sanad init --confirm-native
sanad init --confirm-native --write
sanad sync --write
```

`init` accepts `--confirm-native`, `--automation`, and `--action-ref`. Native support must be confirmed before repository writes. Generated automation requires a full commit of a released Sanad Action; installed release builds provide their own commit by default.

`sync` preserves existing pins while updating coverage for workflow changes. See [Getting started](../../guide/getting-started/) and [Updating dependencies](../../guide/updating/) for the review workflow.

## Update selection

`plan` and `update` accept the following options:

| Flag | Description |
| --- | --- |
| `--action OWNER/REPO[/PATH]` | Select one action or action subpath. |
| `--to REF` | Choose an explicit target; requires `--action`. |
| `--level patch\|minor\|major` | Set the permitted stable update level. |
| `--constraint RANGE` | Set a SemVer range instead of a level. |

`--level` and `--constraint` are mutually exclusive. CLI selection overrides the configured version policy. Explicit targets still obey adoption waiting periods. Branches, partial versions, and prerelease refs remain frozen during automatic selection.

```sh
sanad plan --level major
sanad update --action actions/checkout --to v7.0.1 --write
```

## Verification

```sh
sanad check
sanad check --base origin/main
sanad check --offline
```

The default check verifies online source identity, commit reachability, reference consistency, and local graph integrity. `--base` supplies a trusted revision for checking adoption policy.

`--offline` checks local coverage and graph integrity using the cached official backend. It does not verify source identity, reachability, or upstream freshness. Offline checks cannot be combined with adoption-policy enforcement.

## Repository and authentication

Authentication precedence is `GH_TOKEN`, `GITHUB_TOKEN`, then stored GitHub CLI credentials. The host is discovered from repository context or supplied with `--repo`.

```sh
gh auth login --hostname example.ghe.com
sanad check --repo example.ghe.com/owner/repository
```

GitHub Enterprise Cloud data-residency hosts are supported. GitHub Enterprise Server is not supported by the native backend. Tenant credentials are not reused for public GitHub release discovery.

## Backend setup and recovery

```sh
sanad setup
sanad recover --write
```

Release archives bundle the official backend. Source installations can download the exact checksum-pinned binary on first online use. `SANAD_BACKEND` may point to that exact official binary; it does not permit an arbitrary producer.

An interrupted write leaves `.github/sanad/transaction.json`. Recovery restores the recorded state and refuses to overwrite later edits. Review [Troubleshooting](../../advanced/troubleshooting/#interrupted-transaction) before handling a journal manually.

## Exit codes and shell completion

| Code | Meaning |
| --- | --- |
| `0` | The command succeeded. A plan may still contain pending candidates. |
| `1` | Blocking verification or policy findings. |
| `2` | An operational, parsing, or configuration error. |

Generate completion for `bash`, `zsh`, `fish`, or `powershell`:

```sh
sanad completion bash > sanad.bash
```

See [Reports](../reports/) for machine-readable output, and [Configuration](../configuration/) for policy values.
