+++
title = "CI"
description = "Verify pull requests and automate updates."
template = "page"
+++

Initialize the repository with the [CLI](../getting-started/), then add
`.github/workflows/sanad.yml`:

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

Run `sanad sync --write` and commit the workflow and updated lockfile. Make the
Sanad check required in repository settings. The Action installs its matching
immutable CLI release. You can also pin the Action to that release's full commit.

## Scheduled updates

Generate checking and update workflows with the installed CLI:

```sh
sanad init --automation --write
```

Released builds use their full release commit for generated Action references.
For source builds, pass `--action-ref <FULL_RELEASE_COMMIT>`.
The scheduled workflow maintains one update PR using `contents: write` and
`pull-requests: write`; it does not merge automatically. A GitHub App or PAT can
be supplied when bot-created PRs must trigger additional workflows.

Choose Dependabot or Sanad for scheduled version bumps. Both can use Sanad
verification. With optional first-seen policy, merge observation-only PRs before
expecting later dependency changes to pass. CI uses the trusted `base` revision;
observations introduced only by the PR cannot establish maturity.

The first native lockfile establishes an explicitly reported baseline; review it
before merging. Waiting periods apply to subsequent dependency changes.

The Action supports `check`, `status`, `plan`, `sync`, `update`, and `setup`.
`write: 'true'` is required for `sync` or `update` writes. See the
[Action reference](https://github.com/MohamedElashri/sanad/blob/main/action/README.md).
