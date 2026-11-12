+++
title = "Updating dependencies"
description = "Reconcile workflow edits and apply reviewed version changes."
weight = 20
template = "page"
+++

Use `sync` when workflow contents change and `update` when you want newer
dependency versions. Both preview their proposed repository changes unless you
add `--write`.

## Check the current state

```sh
sanad status
sanad check
```

`status` shows recorded dependencies and workflow coverage. `check` performs
online frozen verification, including source identity and commit reachability.
Status alone is not a verification result.

## Reconcile a workflow edit

After adding an action, removing a workflow, or changing a readable `uses:`
reference:

```sh
sanad sync
sanad sync --write
sanad check
```

Synchronization adds or removes dependency coverage while preserving existing
pins. It does not broadly advance mutable tags or branches. Review every
reported file and commit the workflow edits together with their updated
lockfile.

If adoption policy delays a new dependency, follow the report's decision and
eligible date. Do not edit execution pins to bypass the waiting period.

## Review available updates

```sh
sanad plan
```

By default, Sanad considers newer stable releases within the current major
version. It examines candidates from newest to oldest and selects the highest
version that satisfies policy. A newer pending release does not prevent an older
eligible release from being selected.

| Decision    | Meaning                                                                          | Next action                                                  |
| ----------- | -------------------------------------------------------------------------------- | ------------------------------------------------------------ |
| `eligible`  | The version and every changed dependency satisfy policy.                         | Review and apply the update.                                 |
| `pending`   | A release-age or first-seen waiting period has not elapsed.                      | Record observations if needed, then check the eligible date. |
| `unchanged` | No matching newer release is available, or the requested ref is already tracked. | Keep the current lockfile.                                   |
| `frozen`    | The current ref is not a complete stable version.                                | Choose an explicit target if you want to change it.          |
| `blocked`   | A changed transitive version violates its update level or constraint.            | Review the policy or select another parent version.          |

The review includes transitive changes and affected workflows. A version change
can introduce more than one new commit.

## Apply eligible versions

```sh
sanad update
sanad update --write
sanad check
```

The first command is a preview. The second writes eligible changes and, when
configured, candidate observations. Commit all reported files; an
observation-only change is still useful even when no version advances.

> **Review before merging.** A verified lockfile establishes source identity and
> pinned commits. It does not establish that the action's code is safe or free
> of vulnerabilities.

## Select an action or version range

Limit the operation to one action:

```sh
sanad plan --action actions/checkout
sanad update --action actions/checkout --write
```

Change the permitted update level or use a SemVer constraint:

```sh
sanad plan --level patch
sanad plan --level major
sanad plan --constraint '>= 6, < 8'
```

`--level` and `--constraint` are mutually exclusive. See
[Configuration](../../reference/configuration/) for repository-wide and
per-action policies.

## Choose an explicit target

```sh
sanad update --action actions/checkout --to v7.0.1
sanad update --action actions/checkout --to v7.0.1 --write
```

An explicit target still obeys adoption waiting periods. Branches, partial
versions such as `v7`, and prerelease refs remain frozen during automatic
version selection. Broad mutable-ref relocking is intentionally unavailable
because it can move unrelated pins.

## Verify a proposed change against its base

```sh
sanad check --base origin/main
```

Use a fetched, trusted base revision when evaluating changes with adoption
policy. Observations added only by a pull request cannot make that request's
dependencies mature. See [Waiting periods](../policy/) for the full observation
workflow.

## Reusable workflows

`init` and `sync` resolve remote job-level `uses:` calls to full commit SHAs.
Sanad records the readable source ref, host, and numeric repository/owner
identity in `.github/sanad/workflows.json`. Commit this file with the workflow
edits.

```sh
sanad sync
sanad sync --write
sanad plan --action org/ci/.github/workflows/build.yml --to v1.2.0
sanad update --action org/ci/.github/workflows/build.yml --to v1.2.0 --write
```

Calls tracked from full stable versions participate in automatic version
updates, SemVer policy, release cooldowns, and trusted first-seen waiting
periods. Calls on branches or unlabelled SHAs remain frozen until you select an
explicit target. The caller executes the inline SHA; this tracking file is Sanad
policy metadata.

Online checks verify source identity and that the pinned commit is still
reachable from its recorded ref. An unlabelled SHA must be in the default
branch's history; use an explicit tag or branch when that conservative check
excludes a maintenance branch. Changed full-version tag targets fail
verification. Offline checks only check the recorded SHA and local structure.

Sanad also reads the callee's workflow and native lockfile at that exact commit.
It warns if the workflow lacks structural lockfile coverage. This inspection
does not remotely verify every callee dependency or recursively inspect nested
workflow calls. GitHub applies the callee repository's native lockfile; your
caller's lockfile cannot supply that protection. API failures block the
operation instead of being treated as absent coverage.

## Vendor complete action source

Vendoring copies the entire upstream repository snapshot, including
implementation files, bundled JavaScript, and license files. It redirects action
calls from that repository to the committed same-repository snapshot:

```sh
sanad vendor --action actions/github-script
sanad vendor --action actions/github-script --write
sanad vendor --action actions/github-script --to v8.0.1
sanad vendor --action actions/github-script --to v8.0.1 --write
```

The first copy uses the existing native execution pin. `--to` explicitly
resolves a new upstream ref with the pinned producer. The repository-level
operation covers all subactions in the selected repository and refuses ambiguous
installed versions unless you select a target. Review the full copied code in
the proposed diff. Vendored sources use this explicit update command rather than
automatic `update`.

Source snapshots live under `.github/sanad/vendor/owner/repo/`. The separate
`.github/sanad/vendor.json` records source commits, identities, the producer's
upstream lock graph, file digests, and executable modes. Commit every reported
file. Online verification checks the upstream frozen pins and compares copied
files against the archive of that exact commit. Local checks reject modified,
missing, or unexpected files; edit upstream and re-vendor rather than
customizing a managed snapshot in place. Source trust and adoption waiting
periods also apply to updates.

Archive extraction rejects traversal, links, special files, submodules, and
case-colliding or nonportable paths. The snapshot must fit the 64
MiB/10,000-file limit and the overall 64 MiB managed-input limit. If native
processing would rewrite copied metadata, Sanad refuses the whole change so the
snapshot remains exact. Choose an action compatible with this constraint rather
than silently altering it.

A source snapshot does not vendor runtime downloads, container images, or remote
actions called by a composite; native locks and the protection report still
matter. Vendoring also does not establish compatibility with `act`: test that
runner's execution behavior separately before relying on native lockfile
enforcement.

## Tag families and repositories without releases

For a repository that versions several tools independently, select a literal tag
prefix. Version policy applies to the full stable SemVer after that prefix:

```toml
[actions."org/tools/build"]
tag_prefix = "build/"
level = "minor"
prefer_tags = true
```

An installed `build/v1.2.0` can advance to `build/v1.3.0`, while `deploy/v9.0.0`
and unprefixed versions are excluded. Prefixes are explicit; Sanad does not
guess a monorepo's release conventions. CLI level/constraint overrides retain
the family.

By default candidates come from GitHub Releases. `prefer_tags = true` adds full
stable tags when a project does not publish corresponding releases. Existing
release metadata takes precedence, so a draft or prerelease is not reintroduced
through the tag list. This setting can also be placed in `[update]` and applies
to managed reusable workflow versions.

A tag-only automatic candidate has no GitHub release publication timestamp. With
a release cooldown it is blocked, and Sanad can select an older eligible
release. Commit/tag author dates do not establish publication age for this
fallback. First-seen observations remain available when you want waiting periods
for projects without release records. Explicit `--to` selections retain the
ordinary adoption checks and are separate from automatic catalog selection.

Callee coverage inspection checks each visible direct action against the callee
workflow's own pin set, including inline-SHA consistency. An empty workflow
entry cannot hide missing pins. Local action implementations that were not
downloaded leave coverage unestablished; the result is not an online trust audit
of every transitive dependency or nested reusable workflow.
