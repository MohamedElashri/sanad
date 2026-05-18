# Usage

`sanad` has four workflow commands and one metadata command:

```bash
sanad scan
sanad plan
sanad check
sanad apply
sanad version
```

Global flags:

```bash
--config string   path to config file (default ".sanad.toml")
--format string   output format: table, json, or command-specific formats (default "table")
```

## `sanad scan`

Discover and classify `uses:` entries without contacting GitHub.

```bash
sanad scan
sanad --format json scan
sanad scan --workflows .github/workflows
```

Table output includes file, line, action, ref, kind, pin state, validity, ignore state, ignore rule, and parser error.

JSON output includes the same data plus node path, column, raw scalar text, and source line metadata.

Use `scan` when you want a local inventory.

## `sanad plan`

Resolve actionable refs and show the decisions `sanad` would make.

```bash
GITHUB_TOKEN=... sanad plan
GITHUB_TOKEN=... sanad --format json plan
GITHUB_TOKEN=... sanad plan --out sanad-plan.json
GITHUB_TOKEN=... sanad plan --pr-body-out sanad-pr-body.md
```

`plan` may contact GitHub for GitHub actions and reusable workflows. It reads inline `sanad: ref=...` comments and `.github/sanad.lock.json` when present.

Table output starts with a summary:

```text
Summary:
  2 actions found
  1 already pinned
  1 updates available
  0 pending cooldown
  0 policy violations
  0 skipped
```

Decisions include:

- `unchanged`
- `update`
- `pending-cooldown`
- `skip-local-action`
- `skip-docker-action`
- `skip-ignored`
- `error-invalid`
- `error-unpinned`
- `error-short-sha`
- `error-tag-denied`
- `error-branch-denied`
- `error-reusable-workflow-denied`
- `error-unresolved`
- `error-unsupported-policy`

`--pr-body-out` writes a Markdown summary suitable for `gh pr create --body-file`.

## `sanad check`

Validate workflows against policy. This is the CI enforcement command.

```bash
GITHUB_TOKEN=... sanad check
GITHUB_TOKEN=... sanad --format json check
GITHUB_TOKEN=... sanad check --format sarif
GITHUB_TOKEN=... sanad check --strict
GITHUB_TOKEN=... sanad check --fail-on-updates
GITHUB_TOKEN=... sanad check --strict --allow-pending-cooldown
```

Default behavior:

- Fails on policy violations.
- Fails when mutable unpinned refs still need to be pinned.
- Allows already-pinned managed updates unless stricter flags are set.

Strict flags:

- `--fail-on-updates`: fail when an already-pinned managed action has an eligible update.
- `--strict`: fail on eligible updates and pending cooldown updates.
- `--allow-pending-cooldown`: with `--strict`, allow pending cooldown updates.

Exit code `0` means the check passed. Exit code `1` means policy violations or changes are needed.

SARIF output follows the same pass/fail behavior as table and JSON output. It can be uploaded to GitHub code scanning.

## `sanad apply`

Apply approved updates to workflow files and refresh `.github/sanad.lock.json`.

```bash
GITHUB_TOKEN=... sanad apply --dry-run
GITHUB_TOKEN=... sanad apply --interactive
GITHUB_TOKEN=... sanad apply --yes --write
```

Safe defaults:

- `--dry-run` prints a unified diff and writes nothing.
- `--yes --write` is required for non-interactive writes.
- `--interactive` prompts for ambiguous cases and final confirmation.
- A terminal run without `--yes --write` can use the interactive confirmation path.

Interactive mode can:

- ask for an explicit ref for unpinned actions,
- track a logical ref for unmanaged full-SHA pins,
- pin the current head of a branch ref for the current run,
- skip an entry for the current run,
- leave a policy violation in place.

Workflow rewrites preserve the original file bytes as much as possible and validate that the rewritten YAML parses before writing.

## `sanad version`

Print build metadata:

```bash
sanad version
```

Development builds show `dev`, `unknown`, and `unknown` unless linker metadata was supplied by a release build.

## Authentication

`sanad` reads GitHub tokens from:

1. `GITHUB_TOKEN`
2. `GH_TOKEN`

Tokens are not printed.

Set `[github].api_url` in `.sanad.toml` for GitHub Enterprise API endpoints.

## Exit Codes

```text
0  success
1  policy violation or changes needed in check/apply mode
2  invalid configuration
3  unresolved action reference
4  GitHub API failure
5  rate limit failure
6  unsafe rewrite prevented
7  file system write failure
8  internal error
```
