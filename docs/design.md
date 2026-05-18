# Design

`sanad` is a small CLI with one main job: keep GitHub Actions workflow dependencies pinned to immutable commit SHAs while preserving the logical update channel.

## Command Shape

The command surface is intentionally narrow:

```bash
sanad scan
sanad plan
sanad check
sanad apply
sanad version
```

`scan` is local-only. `plan`, `check`, and `apply` share the same planning pipeline so CI and writes are based on the same decisions.

## Pipeline

The main workflow is:

1. Load config.
2. Discover workflow files.
3. Parse YAML with `gopkg.in/yaml.v3`.
4. Extract scalar `uses:` nodes and source locations.
5. Classify references.
6. Recover metadata from inline comments and lockfile entries.
7. Apply ignore rules.
8. Resolve GitHub refs when needed.
9. Evaluate policy and cooldown.
10. Report decisions.
11. For `apply`, rewrite workflow bytes and update the lockfile.

## Packages

Important packages:

- `internal/actions`: parses and classifies `uses:` values.
- `internal/workflow`: discovers workflow files, extracts `uses:` nodes, rewrites workflow bytes, and writes files atomically.
- `internal/githubresolver`: wraps GitHub API resolution.
- `internal/policy`: evaluates action policy, cooldown, ignore rules, and decision kinds.
- `internal/metadata`: parses inline `sanad` comments and manages `.github/sanad.lock.json`.
- `internal/config`: loads the supported `.sanad.toml` subset.
- `internal/cli`: Cobra commands and command-level reports.

## YAML Strategy

`sanad` uses YAML parsing for discovery and source locations, but it does not serialize the YAML tree back to disk.

Rewrites operate on original file bytes. This preserves unrelated formatting, comments, key order, quoting, line endings, and file permissions as much as possible.

## Metadata Strategy

The workflow file remains readable:

```yaml
- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # sanad: ref=v4
```

The lockfile carries machine-readable state:

```text
.github/sanad.lock.json
```

When both sources are present, they must agree.

## Resolver Strategy

The resolver checks refs in this order:

1. Full SHA commit verification.
2. Tag ref lookup.
3. Annotated tag dereference when needed.
4. Branch ref lookup.
5. Commit timestamp lookup.
6. Release timestamp lookup for tags when available.

The resolver lives in `internal/githubresolver` rather than `internal/github` to avoid a name collision with `github.com/google/go-github/v72/github`.

## Reporting Strategy

Reports expose both human-readable reasons and machine-readable reason codes.

Table output is for humans. JSON output is stable enough for CI consumers. `check --format sarif` emits SARIF for code scanning consumers, and `plan --pr-body-out` writes a Markdown summary for automated update pull requests.

`apply --dry-run` prints unified diffs with one full-file hunk per rewritten workflow file. This keeps diff generation deterministic and dependency-free.

## Current Tradeoffs

- Config parsing is a small local parser, not a full TOML parser.
- Interactive prompts are simple built-in text prompts, not a terminal UI framework.
- Default-branch and latest-release discovery for unpinned refs are not implemented yet.
- The lockfile path is fixed.
