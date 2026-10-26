+++
title = "Sanad documentation"
description = "Set up native GitHub Actions lockfiles, review dependency updates, and verify changes in CI."
+++

Sanad builds on GitHub's native `actions.lock`. GitHub enforces the execution pins; Sanad makes onboarding, controlled updates, and review easier.

## Start with the CLI

Install Sanad, authenticate, and preview your first lockfile from a repository checkout:

```sh
brew install --cask MohamedElashri/sanad/sanad
gh auth login
sanad init --confirm-native
```

[Confirm native support](https://docs.github.com/en/actions/how-tos/secure-your-work/lock-workflow-dependencies) for your repository, review the proposed pins, then apply them with `sanad init --confirm-native --write`. The [installation guide](guide/getting-started/) covers archives, Nix, authentication, and the files to commit.

## Verify pull requests with the Action

After checking out your repository with `fetch-depth: 0`, add this step to a pull-request workflow:

```yaml
- uses: MohamedElashri/sanad@a73376eda34400ec92e51fcefec6ee287871da38 # v1.0.1
  with:
    mode: check
    base: ${{ github.event.pull_request.base.sha }}
```

Run `sanad sync --write` after adding the workflow and commit the updated lockfile. Follow the [CI guide](guide/ci/) for a complete workflow and scheduled update PRs.

## Understand the workflow

| Task | Command | What to review |
| --- | --- | --- |
| Reconcile workflow edits | `sanad sync` | Added or removed coverage; existing pins stay fixed. |
| Inspect newer versions | `sanad plan` | Candidate versions, transitive changes, and policy decisions. |
| Apply eligible updates | `sanad update --write` | Reported file changes and any observation records. |
| Verify a pull request | `sanad check --base REV` | Native verification and trusted-base policy results. |

Commands preview repository edits unless you add `--write`. Continue with [updating dependencies](guide/updating/) or look up a specific option in the [CLI reference](reference/cli/).

## Use Sanad with a coding agent

Install the reusable Sanad skill to give your coding agent the same review and
verification workflow. The [agent guide](guide/agents/) covers Codex, Claude Code,
Cursor, GitHub Copilot, Antigravity, and OpenCode, with installation commands and
example requests.
