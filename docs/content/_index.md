+++
title = "Sanad"
description = "Native Actions lockfiles, with clear updates."
+++

Sanad generates GitHub Actions lockfiles, explains dependency changes, and applies
update policies. GitHub enforces the locked commits when workflows run.

## CLI quickstart

[Verify native lockfile support](https://docs.github.com/en/actions/how-tos/secure-your-work/lock-workflow-dependencies)
for your repository, then install and initialize:

```sh
brew install --cask MohamedElashri/sanad/sanad
gh auth login
sanad init --confirm-native
sanad init --confirm-native --write
sanad check
```

Review the preview before writing. Commit `actions.lock` and `.sanad.toml`.
[Other installation options and daily commands](guide/getting-started/).

## Action quickstart

After checking out your repository with `fetch-depth: 0`, add this step to a
pull-request workflow:

```yaml
- uses: MohamedElashri/sanad@v1.0.0
  with:
    mode: check
    base: ${{ github.event.pull_request.base.sha }}
```

Run `sanad sync --write` to cover the new workflow and commit the updated lockfile.
[Complete workflow and scheduled updates](guide/ci/).
