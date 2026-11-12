# Dependency operations

Read the relevant section when managing reusable workflows, vendoring action
sources, explaining dependencies, or importing an external audit. Use Sanad 1.1+.

## Reusable workflows

`sync` turns a remote workflow call into an inline execution SHA and records its
original ref and identity in `.github/sanad/workflows.json`. Preview and apply
with the same options, and commit both files. Select a workflow by its full path:

```sh
sanad plan --action org/ci/.github/workflows/build.yml
sanad update --action org/ci/.github/workflows/build.yml --to v1.2.0
sanad update --action org/ci/.github/workflows/build.yml --to v1.2.0 --write
```

Full stable versions can update automatically. Branches and unlabelled SHAs stay
frozen until an explicit target is selected. Sanad reads the callee workflow and
native lockfile at the pinned commit to inspect structural coverage. Missing pins,
uninspected local actions, and nested workflow calls limit that result; it is not
an online audit of every callee dependency. API failures block the operation.
The callee's repository supplies its runtime lockfile, not the caller's lockfile.

## Vendored action sources

First verify the installed native baseline. Vendoring copies the whole source
repository, including implementation, bundled JavaScript, and licenses, and
redirects calls to the managed local snapshot. It operates on every subaction in
the selected repository:

```sh
sanad vendor --action owner/action
sanad vendor --action owner/action --write
sanad vendor --action owner/action --to v2.1.0
sanad vendor --action owner/action --to v2.1.0 --write
```

Without `--to`, the initial copy uses the existing execution pin. Ambiguous
installed versions require an explicit target. Use `vendor --to` for subsequent
updates; ordinary `update` does not advance vendor snapshots. Source approvals
and waiting periods still apply.

Review all copied code and commit `.github/sanad/vendor.json`, the snapshot under
`.github/sanad/vendor/`, and every other reported file. Do not edit the managed
snapshot: missing, modified, or extra files block verification or replacement.
Native processing must preserve the upstream metadata exactly. If it would rewrite
that metadata, choose a compatible action instead of patching the snapshot.

Online verification checks frozen upstream pins and compares files with that
commit's archive. Offline verification checks local integrity only. Vendoring does
not include runtime downloads, container images, or remote composite dependencies,
and does not establish compatibility with `act`.

## Explain dependency paths and coverage

```sh
sanad explain actions/upload-artifact --format json
sanad status --format json
```

Use `explanations` to trace direct and transitive dependency paths, recorded
host/owner/repository IDs, and pending decisions. Verify those identities with the
source repository before approving them. Explain/status read stored metadata;
use `check --base REV` for online verification.

`protection` inventories native action pins, inline workflow SHAs, containers,
workspace actions, runner images, tools, and shell commands. Surface unknown
runtime downloads and other boundaries when reporting coverage. A recorded pin
does not establish GitHub enforcement or code safety.

To include zizmor findings for the same checkout:

```sh
zizmor --format sarif . > zizmor.sarif
sanad check --base origin/main --zizmor-report zizmor.sarif
```

Install and run zizmor separately when requested; Sanad only imports its SARIF
2.1.0 output. The report must be inside the repository and describe the revision
being checked. Error findings block checks and writes; warnings remain visible.
Do not reuse a stale audit after changing workflows or describe it as full
dependency coverage.
