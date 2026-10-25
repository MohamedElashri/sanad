+++
title = "GitHub Action"
description = "Modes, inputs, outputs, and examples for Sanad in CI."
weight = 30
template = "page"
+++

Use `MohamedElashri/sanad@v1.0.1` after checking out the repository. The Action verifies the immutable release asset and checks that the installed CLI version matches its own version.

## Minimal check

```yaml
- uses: actions/checkout@v7.0.1
  with:
    fetch-depth: 0
- uses: MohamedElashri/sanad@v1.0.1
  id: sanad
  with:
    mode: check
    base: ${{ github.event.pull_request.base.sha }}
```

Use this inside a pull-request workflow with `contents: read`. Run `sanad sync --write` locally after adding it, then commit the updated lockfile. The [CI guide](../../guide/ci/) includes a complete workflow.

## Modes

| Mode | Purpose | Write behavior |
| --- | --- | --- |
| `check` | Frozen verification and optional trusted-base policy check. | Read-only. |
| `status` | Recorded coverage, dependencies, and readiness. | Read-only. |
| `plan` | Review eligible version updates. | Read-only. |
| `sync` | Reconcile workflow edits while preserving pins. | Preview unless `write: 'true'`. |
| `update` | Select updates and record candidate observations. | Preview unless `write: 'true'`. |
| `setup` | Install and verify the matching CLI and native backend. | Tool installation only. |

Initial onboarding uses the CLI's `init` command. The Action does not expose an initialization mode or a native-support confirmation input.

## Inputs

| Input | Default | Description |
| --- | --- | --- |
| `mode` | `check` | One of the modes above. |
| `write` | `'false'` | Apply repository changes in `sync` or `update` mode. |
| `base` | Empty | Trusted base revision for adoption-policy checking. Valid with `check` only. |
| `config` | `.sanad.toml` | Policy file relative to the selected repository directory. |
| `working-directory` | `.` | Repository directory inside `GITHUB_WORKSPACE`. |
| `token` | `${{ github.token }}` | Credentials for the repository's GitHub host. |

`write` accepts the literal strings `'true'` and `'false'`. Setting it for another mode is an error. Configuration and working-directory paths must remain inside the workspace.

For a repository nested in the checkout:

```yaml
- uses: MohamedElashri/sanad@v1.0.1
  with:
    mode: check
    working-directory: services/example
    config: .sanad.toml
    base: ${{ github.event.pull_request.base.sha }}
```

## Outputs

| Output | Meaning |
| --- | --- |
| `passed` | `'true'` when the command and policy passed. |
| `changed` | Whether the run wrote repository changes. |
| `changed-files` | JSON array of paths actually changed by the run. Empty for previews. |
| `updates` | Number of eligible update decisions. |
| `pending` | Number of pending decisions. |
| `report-path` | Path to the full JSON report on the runner. |
| `pr-body-path` | Path to the Markdown review body on the runner. |
| `sanad-version` | Exact installed Sanad CLI version. |

`setup` publishes `passed` and `sanad-version`; the report outputs apply to report-producing modes. Proposed diffs remain in the report even when a preview's `changed-files` output is empty.

Report paths refer to runner files. They do not upload artifacts by themselves. Preserve them in your own workflow when you need reports after the job ends.

## Preview and write examples

Preview version updates:

```yaml
- uses: MohamedElashri/sanad@v1.0.1
  with:
    mode: plan
```

Apply eligible changes to the checkout:

```yaml
- uses: MohamedElashri/sanad@v1.0.1
  id: update
  with:
    mode: update
    write: 'true'
```

This edits files in the runner checkout. The Action itself does not commit, push, or open a PR. Use [generated scheduled automation](../../guide/ci/#scheduled-update-pull-requests) for that workflow.

## Permissions and authentication

Read-only verification needs `contents: read`. PR creation needs the additional write permissions described in the CI guide. Tokens are masked, and tenant tokens are not sent to the public GitHub release API.

The Action supports the same host boundaries as the CLI. See [Security and design](../../advanced/security-model/) for native enforcement coverage and limitations.
