# Troubleshooting

Use finding codes, stderr, and the full JSON report to distinguish policy outcomes
from operational failures. `doctor` reports diagnostic warnings; success alone
does not establish readiness or frozen verification.

## Repository identity and authentication

Sanad discovers the repository from `--repo`, then `GH_REPO`, then the `origin`
remote. If discovery selects the wrong target or no remote exists, explicitly pass
`--repo [HOST/]OWNER/REPO` on subsequent commands. `--root PATH` selects local
files; it does not set the remote identity or change the shell's directory. Run
Git inspection commands in that same checkout (or use `git -C PATH`).

GitHub.com and GitHub Enterprise Cloud data-residency hosts such as
`example.ghe.com` are supported. GitHub Enterprise Server hosts are unsupported.
For a tenant, use `gh auth login --hostname example.ghe.com` and
`--repo example.ghe.com/owner/repository` when needed. Authentication precedence
is `GH_TOKEN`, `GITHUB_TOKEN`, then stored GitHub CLI credentials. Environment
tokens must belong to the intended host; inspect their presence without printing
their values. Tenant credentials are not reused for public release discovery.

## Backend installation and checksums

The bundled `gh-actions-lock` is an exact, checksum-pinned official binary.
`sanad setup` can download a missing backend for a supported platform. Offline
checks need that binary installed beforehand. `SANAD_BACKEND` may select a local
copy of the same official binary, and prevents automatic downloads when set.

An existing binary with a wrong checksum is rejected; `setup` does not replace
it automatically. Check the selected path and version, preserve anything needed
for diagnosis, and restore the correct binary from the matching release or select
a verified copy. A custom producer, a different backend release, or disabling
checksum checks cannot substitute for the required binary. Do not change the
checksum pins to silence an installation failure.

## Workflow coverage and native limitations

For uncovered workflows or changed references, preview `sanad sync` and inspect
its result before applying. Waiting periods may permit only observation changes,
leaving the lockfile and workflow temporarily out of sync. Report that pending
state and verify again after adoption becomes eligible. Version planning starts
by verifying the existing lockfile; repair coverage before using `plan` or `update`.

For malformed native graphs, restore a reviewed valid lockfile and regenerate
changes rather than inventing IDs or commits. Investigate source-identity changes,
moved refs, unreachable pins, and unknown reachability before accepting a target.
There is no broad accept-moved or mutable-ref refresh shortcut.

The pinned producer can reject actions that generate local sub-actions at runtime.
Retain the repository's existing protection when an upstream limitation prevents
onboarding and report the unsupported dependency. Use the finding and pinned
backend version to assess the limitation; do not create a fallback inline-pinning
scheme or claim that unsupported workflows are covered.

## Interrupted or concurrent writes

An existing `.github/sanad/transaction.json` blocks normal repository operations.
Inspect and preserve current changes before an authorized `sanad recover --write`.
Recovery restores the transaction's recorded state and refuses to overwrite later
edits. Inspect the working tree afterward; its JSON `changed` field does not
describe restored files. Keep the journal out of normal commits.

A "repository changed during planning" error means another edit intervened. Inspect
the current tree and rerun the preview against that state. Do not overwrite the
other edit or reuse an earlier diff as if it were still current.

Legacy `.github/sanad.lock.json`, 0.x configuration, and `# sanad:` comments require
the reviewed [0.x reset workflow](https://melashri.net/sanad/guide/migration/).
Preserve user changes while preparing that reset; native 1.x commands cannot
import the old metadata.

See the [troubleshooting guide](https://melashri.net/sanad/advanced/troubleshooting/)
for additional context.
