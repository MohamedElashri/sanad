+++
title = "CI and automation"
description = "Check pull requests and maintain dependency update PRs."
weight = 30
template = "page"
+++

The Sanad Action installs the CLI version matching its release, verifies the immutable release asset, and publishes a structured report and job summary. Initialize your repository with the [CLI quickstart](../getting-started/) before enabling checks.

## Add a pull-request workflow

Create `.github/workflows/sanad.yml`:

```yaml
name: Sanad
on: pull_request
permissions:
  contents: read
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7.0.1
        with:
          fetch-depth: 0
      - uses: MohamedElashri/sanad@v1.0.0
        with:
          mode: check
          base: ${{ github.event.pull_request.base.sha }}
```

`fetch-depth: 0` makes the trusted base commit available locally. `base` is required when enforcing first-seen policy and also lets Sanad compare changes with the existing baseline.

Run the following locally and commit both the workflow and the resulting lockfile:

```sh
sanad sync --write
sanad check
```

After the workflow succeeds, make the Sanad check required in repository settings. The version tag shown above belongs to an immutable release; you can also pin the Action to that release's full commit.

## Generate workflows automatically

An installed release can generate a checking workflow and a scheduled update workflow:

```sh
sanad init --automation
sanad init --automation --write
```

Generated Action references use the release's full commit. Source builds require an explicit `--action-ref <FULL_RELEASE_COMMIT>`. Sanad refuses to overwrite independently maintained workflow files with different contents.

The generated updater runs on Mondays at 03:00 UTC and also supports manual dispatch. Review the generated schedule and permissions before committing it.

## Scheduled update pull requests

The updater calls Sanad's reusable update workflow. It maintains one PR containing eligible dependency updates or newly recorded observations. It stages only the files reported by Sanad and never enables automatic merging.

| Workflow | Required permissions | Purpose |
| --- | --- | --- |
| Pull-request check | `contents: read` | Verify the proposed dependency graph and policy. |
| Scheduled updater | `contents: write`, `pull-requests: write` | Push the managed update branch and create or update a PR. |

The repository must allow GitHub Actions to create pull requests. The default token may not trigger additional workflows when it creates a PR. Supply an appropriately scoped GitHub App or personal access token when those follow-up checks are required.

## Choose one updater

Use either Dependabot or Sanad for scheduled version changes to avoid competing PRs. Both can use the Sanad verification workflow. A supported Dependabot `cooldown.default-days` setting also applies to Sanad's adoption decisions; see [Configuration](../../reference/configuration/#dependabot-release-age-policy).

## Enforce waiting periods

With optional first-seen policy, the updater can open a PR that changes only `.github/sanad/state.json`. Merge that observation record before expecting a later version update to pass its waiting period.

CI reads observations from the trusted base commit. Records introduced only by the proposed PR do not establish maturity. The initial native lockfile is an explicitly reported baseline and needs review; subsequent dependency changes are subject to the policy.

See [Waiting periods](../policy/) for a worked example.

## Read results

Each check produces annotations for findings, a job summary, and paths to the JSON report and Markdown review body. Failed commands preserve operational reports when a valid report is available.

See the [Action reference](../../reference/action/) for all modes, inputs, outputs, and write controls, or [Reports](../../reference/reports/) to consume results in other automation.
