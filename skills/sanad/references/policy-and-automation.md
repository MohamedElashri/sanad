# Policy and automation

Read this when configuring Sanad policy, adding CI checks, or generating scheduled
updates. Use the repository's existing policy and automation as the starting point.

## Persistent configuration

`sanad config` prints the effective configuration as JSON, including defaults.
The default file is `.sanad.toml`; `--config PATH` selects another path inside the
repository. A missing default file uses defaults; a missing custom file is an
error. Existing files must have `version = 1`. Unknown keys are rejected.

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

These version limits do not replace adoption waiting periods. To add first-seen
policy to the existing TOML file:

```toml
[observation]
minimum_age = "7d"
```

Nonnegative day values and Go durations such as `48h` are supported; omitting the
setting or using `0h` disables this waiting period. Preview commands do not start
the clock. Write and review observation state through `update --write` for version
candidates or `sync --write` for workflow edits. Checks with a nonzero first-seen
period require `--base` even locally.

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
`default-days`; the day value must be an integer from 0 to 36500. The cooldown is
repository-wide. Both release-age and first-seen policy apply to changed direct
and transitive dependencies, and both must pass when enabled. A changed source
identity, ref, or commit cannot borrow another observation's maturity. Initial
native onboarding establishes a reviewed baseline instead of waiting for history
that does not exist yet.

With `check --base REV`, Sanad trusts observations in the base and enforces at
least the base's waiting periods. Lowering a period or backdating state in the
proposed change cannot make its dependencies mature. A plain `check` without a
base does not compare dependency changes for release-age policy; use a trusted
base when checking that policy as well.

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
      - uses: MohamedElashri/sanad@v1.0.1
        with:
          mode: check
          base: ${{ github.event.pull_request.base.sha }}
```

The release tag can be replaced with that release's verified full commit. The
base must be available in the checkout; use the actual PR base, not a revision
introduced only by the PR. Reconcile the new workflow with `sanad sync`, review
and apply with `sanad sync --write`, then verify with the appropriate `check`
command from `SKILL.md`. Commit the workflow and every reported file together.
Making the check required in repository settings is a separate repository change;
do it when requested or report it as the remaining setup step.

## Generated verification and update workflows

For an already-confirmed repository using an installed release:

```sh
sanad init --automation
sanad init --automation --write
```

For first-time onboarding, add `--confirm-native` after confirming native support.
Source builds require `--action-ref FULL_RELEASE_COMMIT` on both commands; replace
the placeholder with the verified full commit of a released Sanad 1.x Action.
Released binaries provide their release commit by default. The generator writes
`.github/workflows/sanad-check.yml` and `.github/workflows/sanad-update.yml` and
refuses to overwrite files with different contents. Reconcile independently
maintained workflows deliberately rather than deleting them to make generation
succeed.

Review the scheduled updater's Monday 03:00 UTC schedule and manual dispatch
trigger. It requires `contents: write`, `pull-requests: write`, and repository
settings allowing Actions to create PRs. It maintains one update PR, stages the
reported changed files, and does not enable automatic merging. An observation-only
PR must reach the trusted base before those observations can establish maturity.
Choose the scheduled updater consistently with the user's existing Dependabot or
Sanad setup to avoid competing version-update PRs.

PRs created with the default `GITHUB_TOKEN` may not trigger follow-up workflows.
When follow-up checks are required, configure an appropriately scoped GitHub App
or personal access token through the repository's normal secret-management process.

## Action modes and reports

Action modes are `check`, `status`, `plan`, `sync`, `update`, and `setup`. `check`
is the default. Only `sync` and `update` accept `write: 'true'`; both preview by
default. `base` is valid only with `check`. `config` is repository-relative and
`working-directory` selects a directory inside the checkout workspace. `token`
defaults to `${{ github.token }}`.

The Action verifies and installs its matching release. Applying changes edits the
runner checkout; the Action alone does not commit, push, or create a PR. Generated
scheduled automation supplies those steps.

Inspect `passed` and the job outcome. Outputs include `changed`, `changed-files`,
`updates`, `pending`, `report-path`, `pr-body-path`, and `sanad-version`. Preview
`changed-files` is empty while JSON still includes proposed diffs. Reports live on
the runner and must be uploaded explicitly if they are needed after the job ends.
`setup` provides installation results rather than repository reports.

See the [configuration](https://melashri.net/sanad/reference/configuration/) and
[Action](https://melashri.net/sanad/reference/action/) references for more detail.
