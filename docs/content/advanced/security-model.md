+++
title = "Security and design"
description = "What is checked, and where."
template = "page"
+++

GitHub’s official producer resolves dependencies and verifies source identity,
commit reachability, and reference consistency. GitHub enforces native pins at
execution. Repository eligibility must be confirmed separately. A successful local
check does not establish runtime entitlement.

Sanad also rejects dangling dependency keys and incomplete transitive workflow
lists locally, including in offline mode. The native producer can reject actions
that generate local sub-actions at runtime; these require an upstream change
before the whole repository can migrate.

A lockfile covers action dependencies, including transitives. Dependencies inside
an external reusable workflow rely on its defining repository’s lockfile. Reports
identify this boundary. Docker image contents, downloaded scripts, package-manager
dependencies, and vulnerability assessment are outside Sanad’s scope.

Sanad generates candidates in isolated metadata snapshots, never executes dependency
code, checks every changed graph entry, and verifies the final candidate with the
native frozen check. Selected updates cannot move unrelated pins. Same-repository
local actions are converted by the native producer to `$/` references.

Optional first-seen state keys include host, numeric source identity, ref, and
commit. A retargeted ref starts a new clock. Changed transitive commits must also
mature. CI reads observations from `--base`, never from untrusted PR changes.
Repository administrators can edit policy and history: this is reviewed Git state,
not a trusted timestamp service. Protect workflows, policy, and state with review.

Writes detect intervening edits and use a recovery journal at
`.github/sanad/transaction.json`. Multi-file writes are not crash-atomic.
`sanad recover --write` restores an interrupted transaction and refuses to overwrite
later edits. A journal indicates an incomplete write; investigate unexpected journals.

The implementation has two packages: `internal/sanad` for policy, snapshots,
transactions, and the native adapter; `internal/cli` for commands and presentation.
The native backend is pinned to `v0.2.0`, the official parser to `v0.0.6`.
Changing either requires producer contract tests. Keep dependency generation in
the upstream tool; do not add a second resolver.
