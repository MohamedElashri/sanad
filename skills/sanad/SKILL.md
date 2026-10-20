---
name: sanad
description: Manage native GitHub Actions lockfiles with Sanad. Use when asked to adopt Sanad, reconcile workflow edits in a repository using Sanad, review or apply action dependency updates, configure Sanad policy or CI automation, or diagnose verification and adoption-policy findings.
license: MIT
---

# Sanad

Sanad manages GitHub's native `.github/workflows/actions.lock`. Use the CLI to
generate and verify execution pins, including transitive action dependencies.
Dependencies inside external reusable workflows use the defining repository's
lockfile; surface the report's boundary warnings when describing coverage.
Verification establishes pinned source identity and graph integrity. It does not
assess vulnerabilities, action code safety, package dependencies, Docker image
contents, or runtime downloads.

## Establish context

Run from the target repository, or pass `--root PATH` to each command.

```sh
sanad version
```

These instructions target Sanad 1.x. If it is missing or reports 0.x, follow the
[installation guide](https://melashri.net/sanad/guide/getting-started/) before
using this workflow. Keep the release's `sanad` and `gh-actions-lock` executables
together on `PATH`. `sanad setup` installs and verifies the exact official backend
when needed; `sanad doctor` diagnoses backend, repository, and authentication issues.
Authentication uses `GH_TOKEN`, then `GITHUB_TOKEN`, then stored `gh` credentials.
For repository/host discovery failures, checksum errors, interrupted writes, or
unsupported actions, read [troubleshooting](references/troubleshooting.md).

Once Sanad 1.x is available, inspect the repository:

```sh
git status --short
sanad status --format json
sanad config
```

Read the existing `.sanad.toml` (or the policy selected by `--config`) and preserve
the user's working changes. Pass the same repository-relative `--config PATH`
throughout when using a custom policy location. Preview
the relevant operation, inspect its findings and proposed file diffs, then repeat
it with `--write` when applying changes is within the user's request. Keep the
same selection flags when writing. A request for review or a plan calls for a
preview only; an authorized fix or update can proceed after reviewing the preview.

## Choose the operation

| Task | Preview | Apply |
| --- | --- | --- |
| Create the first native baseline | `sanad init --confirm-native` | `sanad init --confirm-native --write` |
| Reconcile added, removed, or edited workflows/actions | `sanad sync` | `sanad sync --write` |
| Review newer stable versions | `sanad plan` | No write operation needed. |
| Update eligible dependencies | `sanad update` | `sanad update --write` |

Use `--confirm-native` only after the user has confirmed GitHub native lockfile
support for the repository, or existing repository configuration records that
confirmation. This flag records an assertion; a successful local check cannot
establish GitHub runtime support. Initial pins are a reviewed baseline rather
than updates subject to waiting periods.

`sync` preserves existing pins while reconciling workflow coverage. Use it after
workflow edits; use `update` when the task requires newer dependency versions.

Scope updates to the requested action and version policy:
`--action OWNER/REPO[/PATH]` selects an action repository or subpath.

```sh
sanad plan --action actions/checkout
sanad update --action actions/checkout --write
sanad update --action actions/checkout --to v7.0.1
sanad update --action actions/checkout --to v7.0.1 --write
```

Automatic updates stay within the current major by default. Choose `--level patch`,
`--level minor`, `--level major`, or `--constraint RANGE` when the request calls for
that scope; level and constraint are mutually exclusive. `--to` requires
`--action` and overrides automatic version selection, including configured version
ranges, while still obeying waiting periods. Use it for a requested or explicitly
reviewed target. Branches, commit SHA refs, partial versions, and prerelease refs
remain frozen during automatic selection.

For persistent version limits, per-action overrides, waiting periods, or CI
setup, read [policy and automation](references/policy-and-automation.md).

## Read decisions and failures

Use `--format json` for structured reports. Inspect `success`, `findings`,
`changes`, `decisions`, and `files` along with the process exit code:

- `0`: command succeeded; candidates can still be pending.
- `1`: blocking verification or policy findings.
- `2`: operational, parsing, or configuration error; consult stderr too.

`files` contains proposed diffs even in a preview. For successful `init`, `sync`,
and `update` operations, `changed` records whether repository changes were written.
Recovery restores files without populating that change report; inspect the working
tree after recovery or a failed write. `config`, `version`, and `setup` have their
own output shapes. Review transitive changes as well as direct ones.

For `pending` decisions, explain the reason and `eligible_at` date when present.
Previews do not persist observations. With first-seen policy, an authorized
`sanad update --write` can record `.github/sanad/state.json` without advancing a
version. For pending workflow edits, use `sanad sync --write` to record their
observations. Pending synchronization can leave workflow coverage incomplete;
report that state and rerun `sync` after the waiting period. Observations must
reach the trusted base through the repository's review process before CI can
trust them. Preserve waiting periods and recorded timestamps.
A newer pending release can coexist with an older eligible selection; report
both rather than treating every pending decision as a command failure.

Investigate source-identity changes, replaced repositories, unreachable commits,
and moved refs before choosing an explicitly reviewed target. Generate lockfile
pins with Sanad rather than editing dependency IDs or commits by hand. Sanad 1.x
uses native lockfiles; legacy `.github/sanad.lock.json`, `# sanad:` comments, and
inline SHA management belong to the separate
[0.x reset workflow](https://melashri.net/sanad/guide/migration/).

If `.github/sanad/transaction.json` exists, inspect the working tree and use
`sanad recover --write` when restoring the interrupted operation is authorized.
Recovery refuses to overwrite subsequent edits; preserve those edits and
investigate the journal if recovery fails.

## Verify and report

After applying changes, run `sanad check --base REV` when first-seen policy is
enabled or when evaluating a pull request against adoption policy. First-seen
policy requires a base even for local checks. Choose the actual fetched, trusted
base revision; a revision containing only the proposed changes cannot establish
their trusted history. Otherwise, run `sanad check`.

`sanad check --offline` checks local coverage and graph integrity with an installed
official backend. It rejects `--base` and enabled first-seen policy, and cannot
enforce adoption policy. When online checks cannot run, report the verification limitation
and leave existing policy intact. `status` is not frozen verification.

Review the final working-tree diff. Include every reported managed file in the
change: the native lockfile, the selected policy file, observation state when
present, and workflow or local-action edits (including conversions from `./path` to native
`$/path`). Summarize dependency changes, pending decisions, verification results,
and any blockers. The transaction journal is recovery metadata, not a file to
include in a normal commit. Follow the user's scope for commits and pull requests.

For less common flags, run `sanad COMMAND --help` or read the
[CLI reference](https://melashri.net/sanad/reference/cli/).
