+++
title = "Waiting periods"
description = "Apply release-age and first-seen policy to dependency changes."
weight = 40
template = "page"
+++

Sanad can delay adoption of changed commits in two ways: upstream release age and the time a specific dependency was first observed in your repository. Both apply to changed transitive dependencies as well as the action you selected.

## Choose a policy

| Policy | Clock starts from | Configuration | Persistent state |
| --- | --- | --- | --- |
| Release age | The upstream release, tag, or commit timestamp used for the dependency. | Dependabot `github-actions` cooldown. | No observation file required. |
| First seen | Sanad's recorded observation of an identity, ref, and commit. | `.sanad.toml` observation policy. | Commit `.github/sanad/state.json`. |

If both are enabled, every changed dependency must satisfy both. Neither policy is a vulnerability assessment or a guarantee about upstream code quality.

## Enable first-seen policy

Add this to your versioned `.sanad.toml`:

```toml
[observation]
minimum_age = "7d"
```

The value accepts days such as `7d` or Go duration syntax such as `48h`. Omit the setting, or use a zero duration, when you do not want a first-seen waiting period.

The clock is associated with the host, numeric owner and repository identities, reference, and commit. Retargeting a tag to another commit starts a new clock. A mature direct action cannot make a newly introduced transitive commit mature.

## Record an observation

```sh
sanad plan
sanad update --write
```

A preview reports pending candidates but does not persist an observation. `update --write` records candidate observations even when no version can be applied yet. Review and commit the reported state file through your normal process.

> **Merge observation-only PRs.** CI trusts the observation history already present in the base branch. An unmerged state change does not establish maturity for another pull request.

## Example: a seven-day wait

| Time | Repository event | Result |
| --- | --- | --- |
| Day 1 | The updater first sees a new action commit. | It records an observation; the version remains pending. |
| Day 2 | You merge the observation-only PR. | The base branch now contains the trusted observation recorded on Day 1. |
| Day 8 | The updater evaluates the same identity, ref, and commit. | It can select the update if all other policy checks pass. |

If a transitive dependency changes during the wait, that newly observed commit needs its own waiting period. Sanad keeps multiple candidate histories, so it can still select an older eligible version.

## Check against a trusted base

```sh
git fetch origin main
sanad check --base origin/main
```

CI uses the base's observations and applies at least the waiting period already configured there. Lowering policy or inventing an older observation in a PR does not make that PR's dependency change eligible.

Offline checks cannot enforce this adoption policy. Use an online check with a trusted base revision.

## Initial onboarding

The first native lockfile establishes a reviewed baseline. Sanad reports that the base has no native lockfile and accepts the initial pins rather than making onboarding wait indefinitely for observations that do not yet exist.

Review the initial dependency graph carefully. Waiting periods govern changes after that baseline has been accepted.

## Release-age policy

A single repository-wide Dependabot `github-actions` entry can specify a minimum age:

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

Sanad uses this age when evaluating changed dependency commits. Unsupported cooldown keys or ambiguous multiple `github-actions` entries produce an error. See [Configuration](../../reference/configuration/) for the supported shape.

## Protect the history

Observation history is reviewed Git state, not a trusted timestamp service. Administrators can change it. Protect policy, workflows, lockfiles, and observation records with the same review controls you use for dependency changes.
