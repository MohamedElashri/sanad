# CI Usage

`sanad` is designed to run as a normal CLI in CI. There is no dedicated GitHub Action wrapper to maintain.

## Check Workflow

Use `sanad check` to fail pull requests that introduce mutable or invalid action refs.

```yaml
name: Check pinned actions

on:
  pull_request:
  push:
    branches: [main]

jobs:
  sanad:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683

      - uses: actions/setup-go@93397bea11091df50f3d7e59dc26a7711a8bcfbe
        with:
          go-version: "1.23"

      - name: Install sanad
        run: go install github.com/MohamedElashri/sanad/cmd/sanad@latest

      - name: Check workflow pins
        run: sanad check --format json
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

To publish policy findings as GitHub code scanning annotations, emit SARIF and upload it:

```yaml
      - name: Check workflow pins
        run: sanad check --format sarif > sanad.sarif
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: sanad.sarif
```

## Scheduled Update Workflow

Use `sanad apply --yes --write` to rewrite workflows and update `.github/sanad.lock.json`, then create a pull request when files changed.

```yaml
name: Update pinned actions

on:
  schedule:
    - cron: "0 5 * * 1"
  workflow_dispatch:

permissions:
  contents: write
  pull-requests: write

jobs:
  update-actions:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683
        with:
          fetch-depth: 0

      - uses: actions/setup-go@93397bea11091df50f3d7e59dc26a7711a8bcfbe
        with:
          go-version: "1.23"

      - name: Install sanad
        run: go install github.com/MohamedElashri/sanad/cmd/sanad@latest

      - name: Generate PR body
        run: sanad plan --pr-body-out sanad-pr-body.md
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Apply updates
        run: sanad apply --yes --write
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Create pull request
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          if ! git diff --quiet; then
            git checkout -b sanad/update-action-pins
            git add .github/workflows .github/sanad.lock.json .sanad.toml
            git commit -m "ci: update pinned GitHub Actions"
            git push --force-with-lease origin sanad/update-action-pins
            gh pr create \
              --title "ci: update pinned GitHub Actions" \
              --body-file sanad-pr-body.md \
              --base main \
              --head sanad/update-action-pins
          fi
```

## Useful Modes

Use strict enforcement when managed updates should block CI:

```bash
sanad check --strict
```

Allow cooldown-pending managed updates while still failing eligible updates:

```bash
sanad check --strict --allow-pending-cooldown
```

Fail only on eligible updates for already-managed pins:

```bash
sanad check --fail-on-updates
```

Emit JSON for automation:

```bash
sanad --format json check
sanad --format json plan --out sanad-plan.json
sanad check --format sarif > sanad.sarif
sanad plan --pr-body-out sanad-pr-body.md
```

## Token Guidance

Use `GITHUB_TOKEN` for GitHub Actions. `sanad` also accepts `GH_TOKEN`, but `GITHUB_TOKEN` has priority.

Authenticated requests are recommended even for public repositories because unauthenticated GitHub API limits are low. Private repositories require a token with access to the referenced action repositories.

For GitHub Enterprise, configure the API endpoint in `.sanad.toml`:

```toml
[github]
api_url = "https://github.example.com/api/v3"
```
