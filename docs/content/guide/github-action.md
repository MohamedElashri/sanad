+++
title = "GitHub Action"
description = "Run Sanad directly as an action or call the optional update pull request workflow."
weight = 35
template = "page"
+++

The Sanad action installs the exact CLI release associated with the action, verifies its immutable GitHub release and SHA-256 asset digest, runs Sanad, and translates its JSON report into annotations, a job summary, and step outputs.

## Check pull requests

Check out the repository before running Sanad. Pin both actions to full commit SHAs:

```yaml
name: Check pinned actions

on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  sanad:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd
      - uses: MohamedElashri/sanad@SANAD_FULL_COMMIT_SHA
```

The default `check` is local-only. Set `fresh: "true"` to resolve tracked refs and fail on eligible updates. Set `strict: "true"` to also fail on candidates still inside cooldown.

## Modes

| Mode | Behavior |
| --- | --- |
| `check` | Validate policy; never writes files. |
| `plan` | Resolve refs and generate JSON and Markdown plan files. |
| `apply` | Update pins that track their current logical refs. Preview-only unless `write: "true"`. |
| `upgrade` | Move all managed pins according to `[upgrade]` policy. Preview-only unless `write: "true"`. |
| `setup` | Install Sanad and add it to `PATH` for later `run:` steps. |

Policy remains in `.sanad.toml`. The action deliberately does not expose cooldown, tag and branch rules, workflow paths, or upgrade constraints as separate inputs.

The action outputs `passed`, `changed`, `changed-files`, `violations`, `updates`, `pending-cooldown`, `report-path`, `pr-body-path`, and `sanad-version`. `changed-files` is a JSON array based on content hashes taken before and after an authorized write.

The action verifies that the downloaded CLI reports the same version as the action release before executing a mode. Outputs receive safe defaults if input validation, installation, or report parsing fails.

## Update pull requests

Sanad also publishes an optional reusable workflow. A caller needs only its schedule, explicit write permissions, and the pinned reusable-workflow reference:

```yaml
name: Update pinned actions

on:
  schedule:
    - cron: "0 5 * * 1"
  workflow_dispatch:

jobs:
  sanad:
    permissions:
      contents: write
      pull-requests: write
    uses: MohamedElashri/sanad/.github/workflows/update-pr.yml@SANAD_FULL_COMMIT_SHA
    with:
      upgrade: false
```

The reusable workflow applies tracked-ref updates, commits only `.github/workflows` and `.github/sanad.lock.json`, and creates or updates one branch and pull request. Set `upgrade: true` to also move logical refs according to the repository's upgrade policy.

The repository must allow GitHub Actions to create pull requests. Pull requests created with the default `GITHUB_TOKEN` do not normally trigger another workflow run. If that behavior is required, pass a narrowly scoped GitHub App or personal access token stored as `SANAD_UPDATE_TOKEN`:

```yaml
    secrets:
      token: ${{ secrets.SANAD_UPDATE_TOKEN }}
```

## Boundaries

- GitHub.com is supported; the action currently rejects GitHub Enterprise Server release endpoints.
- Supported release targets are Linux, macOS, and Windows on amd64 and arm64.
- Checkout is never performed by the core action.
- The core action never commits, pushes, creates branches, or opens pull requests.
- `write: "true"` only changes the checked-out workspace.
- Fork pull requests should run local `check`, never a writing mode.
- Cross-repository private actions require a token that can read their source repositories.
- Arbitrary CLI arguments are not accepted. Use `mode: setup` followed by a normal `run:` step for advanced commands.
- The action does not expand Sanad's scope to Docker images, local actions, YAML formatting, or vulnerability scanning.

The action's [standalone README](https://github.com/MohamedElashri/sanad/blob/main/action/README.md) contains the full input/output reference and local test procedure.
