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
  "version": 2,
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
      "candidates": [
        {
          "logical_ref": "v5.0.0",
          "sha": "2222222222222222222222222222222222222222",
          "seen_at": "2026-05-18T00:00:00Z"
        }
      ],
      "resolved_at": "2026-05-18T00:00:00Z",
      "timestamp": "2026-04-20T10:30:00Z",
      "timestamp_source": "release"
    }
  ]
}
```

Entries are keyed by workflow file and YAML node path. Sanad validates schema version, required fields, duplicate entries, full-SHA pins, and pending candidate observations when loading the lockfile.

`pinned_sha` records the SHA currently present in the workflow. `candidates` records newer ref and SHA pairs and their independent local observation times when `cooldown_source = "first-seen"` is configured. Version 1 lockfiles with singular `candidate_sha` and `candidate_seen_at` fields are migrated in memory and written as version 2 on the next authorized lockfile write.

When `sanad apply --write --yes` or `sanad upgrade --write --yes` succeeds, Sanad updates entries for the workflow nodes it evaluated and preserves all other lockfile entries. Read-only commands never write the lockfile. Deletion is reserved for explicit `lock refresh` or `lock prune --write` operations.

## Reconciliation

Sanad treats workflow files as the current source of truth. The lockfile helps recover metadata and preserve cooldown observations, but it should not force users to delete the file after a safe manual edit.

During reconciliation:

- Current workflow action identity wins over lockfile action identity.
- Current workflow full SHA wins over lockfile `pinned_sha`.
- Inline `sanad: ref=...` comments win over lockfile `logical_ref`.
- Lockfile candidate history is preserved while the workflow file, YAML node, and action identity still match. Update planning reuses a first-seen time only when both the candidate logical ref and resolved SHA match; a retargeted tag starts a new observation window.

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

`action-mismatch`, `pin-drift`, `logical-ref-conflict`, and `candidate-drift` can be handled by `lock repair`. `missing-node` entries are preserved by repair and removed only by `lock prune` or a full refresh. Invalid entries, duplicate entries, unsupported lockfile versions, invalid SHA fields, and invalid timestamps are blocking. Malformed JSON is reported as an explicit lockfile load error before reconciliation.

## Repair commands

Inspect lockfile state without writing:

```bash
sanad lock status
sanad --format json lock status
```

Repair safe stale entries:

```bash
sanad lock repair --dry-run
sanad lock repair --write --yes
```

Rebuild the active entry set from current managed workflow pins:

```bash
sanad lock refresh --dry-run
sanad lock refresh --write --yes
```

Remove entries for deleted workflow nodes only:

```bash
sanad lock prune --dry-run
sanad lock prune --write --yes
```

`repair`, `refresh`, and `prune` preview unless `--write` is present. Non-interactive writes also require `--yes`. They do not rewrite workflow YAML and do not contact GitHub.
