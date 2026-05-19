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

When `apply` or `upgrade --write` succeeds, Sanad replaces the lockfile entry set with the currently active managed entries. This removes stale entries for workflow nodes that no longer exist.
