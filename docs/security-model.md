# Security Model

`sanad` reduces GitHub Actions supply-chain risk by replacing mutable workflow dependencies with immutable commit SHAs.

## Core Guarantees

The workflow should run a full 40-character commit SHA:

```yaml
- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683
```

When `sanad` manages a pin, it also records the logical ref:

```yaml
- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # sanad: ref=v4
```

The full SHA is what GitHub Actions executes. The logical ref is only metadata used for future update decisions.

## Resolution

For GitHub action refs, `sanad` resolves:

1. full SHA refs by verifying the commit exists,
2. tags, including annotated tags,
3. branches,
4. matching release, tag, or commit timestamps for cooldown decisions.

The final pinned value is always a commit SHA, not an annotated tag object SHA.

## Cooldown

Cooldown delays adoption of newly resolved upstream commits.

Example:

```toml
cooldown = "14d"
```

If `actions/checkout@v4` moves to a commit from 3 days ago, `sanad` reports `pending-cooldown` and does not rewrite the workflow. Once the commit age reaches the cooldown window, the decision can become `update`.

Timestamp preference for tags:

1. GitHub release published or created time for the tag,
2. annotated tagger time,
3. commit time.

Branch and SHA refs use commit time.

## Denied Or Skipped By Default

Default policy:

- Local actions are skipped.
- Docker actions are skipped.
- Short SHAs are rejected.
- Unpinned GitHub action refs are denied.
- Branch refs are denied.
- Tags are tracked.
- Reusable workflows are allowed.

These defaults are meant to keep non-interactive behavior conservative.

## Metadata Trust

`sanad` reads logical refs from two places:

- inline comments such as `# sanad: ref=v4`,
- `.github/sanad.lock.json`.

When both exist for the same workflow node, they must agree. A conflict is treated as `error-invalid`, because resolving the wrong logical ref could silently update the workflow to an unintended channel.

## Rewrite Safety

`sanad` does not reserialize whole workflow YAML files. It edits the original bytes around the `uses:` scalar, preserving quotes, unrelated comments, line endings, and file permissions as much as possible.

Before writing, it validates:

- edits do not overlap,
- the rewritten workflow parses as YAML,
- atomic file replacement succeeds.

Unsafe rewrites fail with exit code `6`.

## Token Handling

GitHub tokens are read from:

1. `GITHUB_TOKEN`
2. `GH_TOKEN`

Tokens are used only for API authentication and are never printed by command output.

For local shell usage, prefer reusing the GitHub CLI token instead of pasting a token into your terminal:

```bash
GITHUB_TOKEN=$(gh auth token) sanad plan
```

## Current Limits

- `scan` is local-only and does not verify refs against GitHub.
- The CLI currently uses the public GitHub API base URL.
- `[security]` config keys in the example file are reserved and not yet parsed.
- `updates.unpinned = "default-branch"` and `updates.unpinned = "latest-release"` are policy values, but the current resolver does not implement default-branch or latest-release discovery.
