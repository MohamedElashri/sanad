+++
title = "Waiting periods"
description = "Apply release-age and first-seen policy to dependency changes."
weight = 40
template = "page"
+++

Sanad can delay adoption of changed commits in two ways: upstream release age
and the time a specific dependency was first observed in your repository. Both
apply to changed transitive dependencies as well as the action you selected.

## Choose a policy

| Policy      | Clock starts from                                                                                                                    | Configuration                         | Persistent state                   |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------- | ---------------------------------- |
| Release age | The later of the release publication and the locked commit's committer timestamp; the commit timestamp alone when no release exists. | Dependabot `github-actions` cooldown. | No observation file required.      |
| First seen  | Sanad's recorded observation of an identity, ref, and commit.                                                                        | `.sanad.toml` observation policy.     | Commit `.github/sanad/state.json`. |

If both are enabled, every changed dependency must satisfy both. Neither policy
is a vulnerability assessment or a guarantee about upstream code quality.

## Enable first-seen policy

Add this to your versioned `.sanad.toml`:

```toml
[observation]
minimum_age = "7d"
```

The value accepts days such as `7d` or Go duration syntax such as `48h`. Omit
the setting, or use a zero duration, when you do not want a first-seen waiting
period.

The clock is associated with the host, numeric owner and repository identities,
reference, and commit. Retargeting a tag to another commit starts a new clock. A
mature direct action cannot make a newly introduced transitive commit mature.

## Record an observation

```sh
sanad plan
sanad update --write
```

A preview reports pending candidates but does not persist an observation.
`update --write` records candidate observations even when no version can be
applied yet. Review and commit the reported state file through your normal
process.

> **Merge observation-only PRs.** CI trusts the observation history already
> present in the base branch. An unmerged state change does not establish
> maturity for another pull request.

## Example: a seven-day wait

| Time  | Repository event                                          | Result                                                                  |
| ----- | --------------------------------------------------------- | ----------------------------------------------------------------------- |
| Day 1 | The updater first sees a new action commit.               | It records an observation; the version remains pending.                 |
| Day 2 | You merge the observation-only PR.                        | The base branch now contains the trusted observation recorded on Day 1. |
| Day 8 | The updater evaluates the same identity, ref, and commit. | It can select the update if all other policy checks pass.               |

If a transitive dependency changes during the wait, that newly observed commit
needs its own waiting period. Sanad keeps multiple candidate histories, so it
can still select an older eligible version.

## Check against a trusted base

```sh
git fetch origin main
sanad check --base origin/main
```

CI uses the base's observations and applies at least the waiting period already
configured there. Lowering policy or inventing an older observation in a PR does
not make that PR's dependency change eligible.

Offline checks cannot enforce this adoption policy. Use an online check with a
trusted base revision.

## Initial onboarding

The first native lockfile establishes a reviewed baseline. Sanad reports that
the base has no native lockfile and accepts the initial pins rather than making
onboarding wait indefinitely for observations that do not yet exist.

Review the initial dependency graph carefully. Waiting periods govern changes
after that baseline has been accepted.

## Release-age policy

A single repository-wide Dependabot `github-actions` entry can specify a minimum
age:

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

Sanad uses this age when evaluating changed dependency commits. Unsupported
cooldown keys or ambiguous multiple `github-actions` entries produce an error.
See [Configuration](../../reference/configuration/) for the supported shape.

An old release cannot make a commit with a newer committer timestamp mature.
However, Git commit timestamps are supplied by upstream authors, and GitHub's
release publication time does not record later tag retargeting. Release-age
policy therefore cannot prove how long a particular ref has pointed to a commit.
Enable first-seen policy when adoption must wait after Sanad first observes that
exact identity, ref, and commit.

## Protect the history

Observation history is reviewed Git state, not a trusted timestamp service.
Administrators can change it. Protect policy, workflows, lockfiles, and
observation records with the same review controls you use for dependency
changes.

## Approved sources

You can require reviewed repository identities for every direct action,
transitive action, and managed reusable workflow:

```toml
[trust]
require_approval = true

[trust.repositories."github.com/actions/checkout"]
owner_id = 44036562
repo_id = 197814629
```

Use `sanad explain` to see the recorded host and numeric owner/repository IDs,
verify them with the source repository, and approve the policy change through
review. The example IDs identify actions/checkout; verify current ownership
before adopting them. Entries apply to all action subdirectories and reusable
workflows in that repository. Names and hosts compare without case; wildcards
are not used. A listed source remains bound to its IDs even when
`require_approval` is false.

Unapproved sources block synchronization and onboarding. During version
selection, Sanad rejects a candidate introducing an unapproved transitive source
and tries the next candidate. Existing native organization allowlists still
apply at runtime.

Online checks with source trust policy require `--base`. The trusted base's
approvals apply in addition to current policy, so a dependency PR cannot approve
its own new repository or replace a source identity. Merge approval changes
first, then introduce the dependency. Offline checks cannot establish source
trust.
