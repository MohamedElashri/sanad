+++
title = "CI Usage"
description = "Run Sanad in GitHub Actions to enforce pinned workflow dependencies and automate update pull requests."
weight = 40
template = "page"
+++

Use `sanad audit check` to fail pull requests that introduce mutable or invalid action refs.

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
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd

      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff
        with:
          go-version: "1.26.x"

      - name: Install sanad
        run: go install github.com/MohamedElashri/sanad/cmd/sanad@latest

      - name: Check workflow pins
        run: sanad audit check --format json
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## SARIF

To publish findings as GitHub code scanning annotations:

```yaml
      - name: Check workflow pins
        run: sanad audit check --format sarif > sanad.sarif
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - uses: github/codeql-action/upload-sarif@v4
        if: always()
        with:
          sarif_file: sanad.sarif
```

## Automated update pull requests

Use `sanad audit plan --pr-body-out` and `sanad update apply --yes --write`, then create a pull request if files changed.

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
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd
        with:
          fetch-depth: 0

      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff
        with:
          go-version: "1.26.x"

      - run: go install github.com/MohamedElashri/sanad/cmd/sanad@latest
      - run: sanad audit plan --pr-body-out sanad-pr-body.md
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      - run: sanad update apply --yes --write
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
