# Policy and automation

Read this when configuring Sanad policy, adding CI checks, or generating
scheduled updates. Use the repository's existing policy and automation as the
starting point.

## Persistent configuration

`sanad config` prints the effective configuration as JSON, including defaults.
The default file is `.sanad.toml`; `--config PATH` selects another path inside
the repository. A missing default file uses defaults; a missing custom file is
an error. Existing files must have `version = 1`. Unknown keys are rejected.

For a repository whose native support has already been confirmed:

```toml
version = 1

[native]
enabled = true

[update]
level = "minor"

[actions."actions/checkout"]
constraint = ">= 6, < 8"

[actions."actions/setup-node"]
level = "patch"
```

`patch` selects newer stable versions in the same major and minor; `minor` stays
in the same major; `major` permits any newer stable version. A `constraint`
replaces a `level`; setting both in one policy is invalid. Per-action policies
replace the global version limit, with an exact action subpath policy taking
precedence over its repository policy. CLI `--level` or `--constraint` overrides
both. Explicit `--to` targets override automatic version selection.

For independently versioned subactions, set an explicit tag family:

```toml
[actions."org/tools/build"]
tag_prefix = "build/"
level = "minor"
prefer_tags = true
```

This allows `build/v1.2.0` to advance to `build/v1.3.0`. `prefer_tags` adds
stable tags to the release catalog; it does not override draft/prerelease
metadata. Level/constraint overrides retain inherited tag-family and catalog
settings. An explicit `prefer_tags = false` disables inherited tag fallback.
Shared-ref subactions must satisfy all selected policies. Tag-only automatic
candidates cannot satisfy a release cooldown without a release publication
timestamp.

These version limits do not replace adoption waiting periods. To add first-seen
policy to the existing TOML file:

```toml
[observation]
minimum_age = "7d"
```

Nonnegative day values and Go durations such as `48h` are supported; omitting
the setting or using `0h` disables this waiting period. Preview commands do not
start the clock. Write and review observation state through `update --write` for
version candidates or `sync --write` for workflow edits. Checks with a nonzero
first-seen period require `--base` even locally.

For release-age policy, Sanad reads `.github/dependabot.yml` or
`.github/dependabot.yaml` (`.yml` wins if both exist). Preserve other ecosystems
when editing the file. The supported GitHub Actions entry has this shape:

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

Sanad rejects multiple `github-actions` entries and cooldown keys other than
`default-days`; the day value must be an integer from 0 to 36500. The cooldown
is repository-wide. Both release-age and first-seen policy apply to changed
direct and transitive dependencies, and both must pass when enabled. A changed
source identity, ref, or commit cannot borrow another observation's maturity.
Initial native onboarding establishes a reviewed baseline instead of waiting for
history that does not exist yet.

With `check --base REV`, Sanad trusts observations in the base and enforces at
least the base's waiting periods. Lowering a period or backdating state in the
proposed change cannot make its dependencies mature. A plain `check` without a
base does not compare dependency changes for release-age policy; use a trusted
base when checking that policy as well.

## Publisher and source policies

List all distributed metadata files, including exports unused by local
workflows:

```toml
[publisher]
actions = ["action.yml", "action/action.yml"]
```

Remote calls in these files must retain full SHAs; `$/` paths are forbidden
because they resolve in the consumer's repository. If native normalization
conflicts with this contract, use a separate internal composite rather than
removing the guard or rewriting protected metadata after generation.

Require approved identities for every action, transitive source, and reusable
workflow with `[trust] require_approval = true`. Inspect `sanad explain` and
verify the source's current numeric IDs before adding an entry such as:

```toml
[trust]
require_approval = true

[trust.repositories."github.com/actions/checkout"]
owner_id = 44036562
repo_id = 197814629
```

Verify the example IDs before adopting them. Approval covers that repository's
subactions and workflows. Listed IDs remain enforced with
`require_approval = false`. Merge new-source approvals before introducing that
source: online checks enforce both current and trusted-base policy, so a
dependency PR cannot approve itself. Use `check --base REV` whenever source
approvals are configured.

## Pull-request checks

Initialize the native baseline with the CLI before adding CI checks. The GitHub
Action has no `init` mode or native-support confirmation input. A minimal
pull-request workflow is:

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
      - uses: MohamedElashri/sanad@v1.1.0
        with:
          mode: check
          base: ${{ github.event.pull_request.base.sha }}
```

The release tag can be replaced with that release's verified full commit. The
base must be available in the checkout; use the actual PR base, not a revision
introduced only by the PR. Reconcile the new workflow with `sanad sync`, review
and apply with `sanad sync --write`, then verify with the appropriate `check`
command from `SKILL.md`. Commit the workflow and every reported file together.
Making the check required in repository settings is a separate repository
change; do it when requested or report it as the remaining setup step.

## Generated verification and update workflows

For an already-confirmed repository using an installed release:

```sh
sanad init --automation
sanad init --automation --write
```

For first-time onboarding, add `--confirm-native` after confirming native
support. Source builds require `--action-ref FULL_RELEASE_COMMIT` on both
commands; replace the placeholder with the verified full commit of a released
Sanad 1.x Action. Released binaries provide their release commit by default. The
generator writes `.github/workflows/sanad-check.yml` and
`.github/workflows/sanad-update.yml` and refuses to overwrite files with
different contents. Reconcile independently maintained workflows deliberately
rather than deleting them to make generation succeed.

Review the scheduled updater's Monday 03:00 UTC schedule and manual dispatch
trigger. It declares `contents: write` and `pull-requests: write`. It maintains
one update PR, stages the reported changed files, and does not enable automatic
merging. An observation-only PR must reach the trusted base before those
observations can establish maturity. Choose the scheduled updater consistently
with the user's existing Dependabot or Sanad setup to avoid competing
version-update PRs.

Generated automation expects a `SANAD_TOKEN` repository secret containing a
fine-grained personal access token with Contents, Workflows, and Pull requests
write permissions, limited to the target repository. A caller may instead
generate and pass a GitHub App installation token with those permissions. The
reusable updater requires this credential and rejects the built-in
`GITHUB_TOKEN`, which cannot push workflow edits.

## Action modes and reports

Action modes are `check`, `status`, `plan`, `sync`, `update`, and `setup`.
`check` is the default. Only `sync` and `update` accept `write: 'true'`; both
preview by default. `base` is valid only with `check`. `config` is
repository-relative and `working-directory` selects a directory inside the
checkout workspace. `token` defaults to `${{ github.token }}`.

The Action verifies and installs its matching release. Applying changes edits
the runner checkout; the Action alone does not commit, push, or create a PR.
Generated scheduled automation supplies those steps.

Inspect `passed` and the job outcome. Outputs include `changed`,
`changed-files`, `updates`, `pending`, `report-path`, `pr-body-path`, and
`sanad-version`. Preview `changed-files` is empty while JSON still includes
proposed diffs. Reports live on the runner and must be uploaded explicitly if
they are needed after the job ends. `setup` provides installation results rather
than repository reports.

See the [configuration](https://melashri.net/sanad/reference/configuration/) and
[Action](https://melashri.net/sanad/reference/action/) references for more
detail.

## Local hooks and release artifacts

Use `sanad check --offline --coverage-only` for developer hooks when an official
backend is already installed. This explicit mode skips waiting periods and
source approvals, validates local graph coverage, and rejects `--base`. Keep
online `check --base REV` in CI. The repository ships Makefile examples and a
system-language pre-commit hook; pin the hook to a reviewed release commit and
keep the matching CLI/backend on `PATH`.

Sanad 1.1 release archives include SPDX inventories for the compiled
CLI/backend; the release also includes the Action's production npm inventory.
Build provenance covers the archives, checksums, and inventories. When artifact
verification is part of the task, verify the downloaded archive with:

```sh
gh attestation verify sanad_1.1.0_Linux_x86_64.tar.gz --repo MohamedElashri/sanad
```

These inventories do not cover arbitrary runtime downloads or prove code safety.
Normal token-only CLI operation does not require GitHub CLI.
