+++
title = "Lockfile Reference"
description = "How .github/sanad.lock.json stores machine-readable tracking metadata."
weight = 30
template = "page"
+++

The lockfile is optional but recommended. It keeps machine-readable metadata beside the human-readable workflow comments.

Default path:

```text
.github/sanad.lock.json
```

Example:

```json
{
  "version": 1,
  "generated_by": "sanad",
  "entries": [
    {
      "file": ".github/workflows/ci.yml",
      "node": "jobs.test.steps[0].uses",
      "owner": "actions",
      "repo": "checkout",
      "path": "",
      "kind": "github-action",
      "logical_ref": "v4",
      "pinned_sha": "11bd71901bbe5b1630ceea73d27597364c9af683",
      "candidate_sha": "2222222222222222222222222222222222222222",
      "candidate_seen_at": "2026-05-18T00:00:00Z",
      "resolved_at": "2026-05-18T00:00:00Z",
      "timestamp": "2026-04-20T10:30:00Z",
      "timestamp_source": "release"
    }
  ]
}
```

Entries are keyed by workflow file and YAML node path. Sanad validates schema version, required fields, duplicate entries, full-SHA pins, and pending candidate observations when loading the lockfile.

`pinned_sha` records the SHA currently present in the workflow when one exists. `candidate_sha` and `candidate_seen_at` record a newer resolved target while it is cooling down; Sanad uses that local observation time when `cooldown_source = "first-seen"` is configured.

When `sanad update apply --yes --write` or `sanad update upgrade --write` succeeds, Sanad replaces the lockfile entry set with the currently active managed entries. This removes stale entries for workflow nodes that no longer exist.

## Reconciliation

Sanad treats workflow files as the current source of truth. The lockfile helps recover metadata and preserve cooldown observations, but it should not force users to delete the file after a safe manual edit.

During reconciliation:

- Current workflow action identity wins over lockfile action identity.
- Current workflow full SHA wins over lockfile `pinned_sha`.
- Inline `sanad: ref=...` comments win over lockfile `logical_ref`.
- Lockfile candidate history is preserved only when it still belongs to the same workflow file, YAML node, action identity, and logical ref. Update planning reuses a first-seen candidate time only when the resolved candidate SHA still matches the recorded candidate.

Reconciliation diagnostics use these status values:

| Status | Meaning |
| --- | --- |
| `matched` | The lockfile entry matches the current workflow node. |
| `missing-node` | The lockfile entry points at a workflow node that no longer exists. |
| `action-mismatch` | The workflow node now references a different action identity. |
| `pin-drift` | The workflow SHA differs from the lockfile `pinned_sha`. |
| `logical-ref-conflict` | The inline comment ref and lockfile ref disagree. |
| `candidate-drift` | Pending candidate history no longer belongs to the current action metadata. |
| `invalid` | A lockfile entry or inline comment is invalid. |
| `duplicate` | More than one lockfile entry uses the same workflow file and YAML node key. |

`missing-node`, `action-mismatch`, `pin-drift`, `logical-ref-conflict`, and `candidate-drift` are repairable when the rest of the lockfile is valid. Invalid entries, duplicate entries, unsupported lockfile versions, invalid SHA fields, and invalid timestamps are blocking. Malformed JSON is reported as an explicit lockfile load error before reconciliation.

## Repair commands

Inspect lockfile state without writing:

```bash
sanad lock status
sanad --format json lock status
```

Repair safe stale entries:

```bash
sanad lock repair --dry-run
sanad lock repair --write
```

Rebuild the active entry set from current managed workflow pins:

```bash
sanad lock refresh --dry-run
sanad lock refresh --write
```

Remove entries for deleted workflow nodes only:

```bash
sanad lock prune --dry-run
sanad lock prune --write
```

`repair`, `refresh`, and `prune` are dry-run unless `--write` is present. They do not rewrite workflow YAML and do not contact GitHub.
