# Lockfile

`sanad` uses `.github/sanad.lock.json` as machine-readable metadata for managed workflow pins.

Inline comments are useful for humans:

```yaml
- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # sanad: ref=v4
```

The lockfile is useful for deterministic planning and non-interactive operation.

## Path

Default path:

```text
.github/sanad.lock.json
```

The path is currently fixed.

## Schema

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
      "resolved_at": "2026-05-18T00:00:00Z",
      "timestamp": "2026-04-20T10:30:00Z",
      "timestamp_source": "release"
    }
  ]
}
```

## Required Fields

Each entry must include:

- `file`
- `node`
- `owner`
- `repo`
- `kind`
- `logical_ref`
- `pinned_sha`

`pinned_sha` must be a full 40-character SHA.

Optional fields:

- `path`
- `resolved_at`
- `timestamp`
- `timestamp_source`

Current timestamp sources are `release`, `tag`, and `commit`.

## Matching

Entries are matched by workflow file path and stable YAML node path:

```text
file + node
```

Example node path:

```text
jobs.test.steps[0].uses
```

## Updates

`sanad apply --yes --write` writes the active managed set to the lockfile.

Stale entries are removed by replacement: entries that are no longer active in discovered workflows are not preserved in the new lockfile.

The file is written as deterministic indented JSON with mode `0600`.

## Metadata Conflicts

If an inline comment and lockfile entry both exist for a workflow node, their logical refs must match.

This is valid:

```yaml
- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # sanad: ref=v4
```

```json
{
  "logical_ref": "v4"
}
```

This is invalid:

```yaml
- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # sanad: ref=v3
```

```json
{
  "logical_ref": "v4"
}
```

A conflict produces `error-invalid`.
