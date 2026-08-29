# Sanad GitHub Action

This directory contains the source, tests, and committed JavaScript bundle for the Sanad GitHub Action. Normal users invoke the action from the repository root:

```yaml
permissions:
  contents: read

steps:
  - uses: actions/checkout@FULL_COMMIT_SHA
  - uses: MohamedElashri/sanad@SANAD_FULL_COMMIT_SHA
```

The action installs the Sanad CLI version matching its own release, verifies the immutable GitHub release and the asset's SHA-256 digest, checks the installed CLI version, and then translates Sanad's JSON report into annotations, a job summary, and step outputs.

## Modes

| Mode | Writes files | Description |
| --- | --- | --- |
| `check` | No | Validate the checked-out workflows. This is the default. |
| `plan` | No | Resolve refs and produce JSON and Markdown plan files. |
| `apply` | Opt-in | Refresh pins that track their current logical refs. |
| `upgrade` | Opt-in | Move managed logical refs according to `[upgrade]` policy. |
| `setup` | No | Install Sanad and add it to `PATH` for later steps. |

`apply` and `upgrade` are previews unless `write: "true"` is supplied. Sanad policy belongs in `.sanad.toml`; the action intentionally does not duplicate policy settings as action inputs.

### Fresh pull-request checks

```yaml
- uses: MohamedElashri/sanad@SANAD_FULL_COMMIT_SHA
  with:
    fresh: "true"
```

`fresh` resolves tracked refs and fails on eligible updates. `strict` implies `fresh` and also fails when an update is still inside its cooldown period.

### Apply updates in the workspace

```yaml
- id: sanad
  uses: MohamedElashri/sanad@SANAD_FULL_COMMIT_SHA
  with:
    mode: apply
    write: "true"

- name: Inspect changed files
  env:
    CHANGED_FILES: ${{ steps.sanad.outputs.changed-files }}
  run: printf '%s\n' "$CHANGED_FILES"
```

The core action never commits, pushes, or creates a pull request. Use the repository's reusable [`update-pr.yml`](../.github/workflows/update-pr.yml) workflow when that complete workflow is desired.

## Inputs

| Input | Default | Meaning |
| --- | --- | --- |
| `mode` | `check` | `check`, `plan`, `apply`, `upgrade`, or `setup`. |
| `config` | `.sanad.toml` | Config path relative to `working-directory`. The file may be absent to use built-in defaults. |
| `working-directory` | `.` | Repository directory relative to `GITHUB_WORKSPACE`. |
| `fresh` | `false` | Resolve tracked refs during `check`. |
| `strict` | `false` | Also fail cooldown-pending candidates during `check`; implies `fresh`. |
| `write` | `false` | Permit `apply` or `upgrade` to modify managed files. |
| `token` | `github.token` | Token used for release lookup and GitHub ref resolution. |

Boolean inputs accept only `"true"` or `"false"`. `fresh` and `strict` are valid only for `check`; `write` is valid only for `apply` and `upgrade`.

## Outputs

| Output | Meaning |
| --- | --- |
| `passed` | Whether the command completed successfully and, for `check`, passed policy. |
| `changed` | Whether an authorized write changed a managed file. |
| `changed-files` | JSON array of changed managed paths. |
| `violations` | Policy violation count, or blocked-decision count for `upgrade`. |
| `updates` | Available or applied update count reported by Sanad. |
| `pending-cooldown` | Candidate count still inside cooldown. |
| `report-path` | Temporary path to the complete JSON report. |
| `pr-body-path` | Temporary Markdown plan path for `plan` and `apply`. |
| `sanad-version` | Exact CLI version validated and executed. |

Outputs receive safe defaults even when setup, input validation, installation, or report parsing fails. A failed Sanad command still fails the action step.

## Permissions and boundaries

- Check out the caller repository before invoking any mode, including `setup`.
- `contents: read` is sufficient for public actions and ordinary checks.
- Private cross-repository actions need a token that can read their source repositories.
- Linux, macOS, and Windows runners on amd64 or arm64 are supported.
- GitHub.com release installation is supported. GitHub Enterprise Server is not currently supported.
- Fork pull requests should use a non-writing `check`; do not expose a write-capable token to untrusted code.
- Arbitrary CLI arguments are intentionally unavailable. Use `mode: setup`, then invoke `sanad` in a subsequent `run:` step.
- The action does not expand Sanad into a YAML formatter, vulnerability scanner, Docker updater, or local-action rewriter.

## Local development and tests

Node 24 or newer and the repository's supported Go toolchain are required.

```bash
cd action
npm ci
npm test
npm run test:integration
npm run check-dist
```

`npm test` runs the offline unit and consistency tests. `npm run test:integration` builds a versioned local Sanad binary, runs the committed action bundle against an isolated temporary repository, and exercises setup, a failing check, plan, apply, upgrade, and a passing check. The plan/apply/upgrade portion contacts GitHub's public API; set `SANAD_ACTION_TEST_TOKEN` if unauthenticated API limits are too low.

`npm run test:local` runs the complete sequence. The test-only binary override is accepted only when `SANAD_ACTION_TESTING=true`; normal action execution always uses the verified release installer.

After changing files under `src/`, rebuild and commit both files under `dist/`:

```bash
npm run build
```

The root [`action.yml`](../action.yml) is the public action metadata. The copy in this directory supports the reusable workflow's `$/action` self-reference. A unit test requires the two metadata files to remain identical except for the bundle path.
