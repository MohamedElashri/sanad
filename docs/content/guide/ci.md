+++
title = "CI"
description = "Verify changes and automate updates."
template = "page"
+++

Generate workflows after a 1.x release is published:

```sh
sanad init --automation --action-ref <RELEASE_COMMIT> --write
```

The generated PR check uses the trusted base commit and reads repository contents.
The first native lockfile establishes an explicitly reported baseline; review it
before merging. Waiting periods apply to subsequent dependency changes.
Make it required in repository settings. Scheduled updates use a separate workflow
with `contents: write` and `pull-requests: write` to maintain one PR. No automatic
merge is enabled. A GitHub App or PAT can be supplied if bot-created PRs must
trigger additional workflows.

Use Dependabot or Sanad for scheduled version bumps. Both can use Sanad verification.
First-seen policy needs observation-only changes merged to the base branch before
later dependency changes pass. Temporary caches do not persist observation clocks.

The Action supports `check`, `status`, `plan`, `sync`, `update`, and `setup`.
`write: 'true'` is required for `sync` or `update` writes. `base` is required when
checking first-seen policy; checkout with `fetch-depth: 0`. See the
[Action README](https://github.com/MohamedElashri/sanad/blob/main/action/README.md).
