+++
title = "Internals"
description = "How Sanad discovers, resolves, evaluates, rewrites, and reports workflow dependencies."
weight = 20
template = "page"
+++

Sanad keeps command behavior centered on one shared pipeline:

1. Load config.
2. Discover workflow files.
3. Parse YAML with `gopkg.in/yaml.v3`.
4. Extract scalar `uses:` nodes and source locations.
5. Classify references.
6. Recover metadata from inline comments and the lockfile.
7. Apply ignore rules.
8. Resolve GitHub refs when needed.
9. Evaluate policy and cooldown.
10. Report decisions.
11. For writes, rewrite workflow bytes and update the lockfile.

## Important packages

`internal/actions` parses and classifies `uses:` values.

`internal/workflow` discovers workflow files, extracts `uses:` nodes, rewrites workflow bytes, and writes files atomically.

`internal/githubresolver` wraps GitHub API resolution.

`internal/policy` evaluates action policy, cooldown, ignore rules, and decision kinds.

`internal/metadata` parses inline `sanad` comments and manages `.github/sanad.lock.json`.

`internal/config` loads and validates `.sanad.toml`.

`internal/cli` contains Cobra commands and command-level reports.

## YAML strategy

Sanad parses YAML for structure and source locations, but it does not serialize the YAML tree back to disk.

Rewrites operate on original file bytes. This preserves unrelated formatting, comments, key order, quoting, line endings, and file permissions as much as possible.

## Resolver strategy

Resolution checks full SHA commits, tags, annotated tags, branches, commit timestamps, and release timestamps. Automatic upgrades paginate stable GitHub releases, order them by SemVer, apply repository and per-action constraints, and resolve candidate tags through the source repository. Cooldown uses upstream timestamps by default, or each lockfile candidate's local observation time when `cooldown_source = "first-seen"`.

The resolver package is named `githubresolver` to avoid colliding with `github.com/google/go-github/v72/github`.
