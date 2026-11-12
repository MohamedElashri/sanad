+++
title = "Reports and exit codes"
description = "Read review summaries and consume structured results in automation."
weight = 40
template = "page"
+++

Sanad uses one versioned report format for its repository commands. Use readable output during review and JSON when another tool needs to consume the result.

## Output formats

```sh
sanad plan
sanad plan --format markdown
sanad plan --format json > sanad-plan.json
```

The default `table` format shows a styled terminal report with headings, command results, native support, verification, workflow coverage, findings, and dependency tables. Cyan identifies headings and paths, green successful or eligible results, yellow warnings and pending or frozen updates, and red failures and errors. Labels remain visible when color is disabled. A successful command does not establish that native enforcement is enabled; read the separate native support and verification fields.

Tables wrap cells to the terminal width. When a table cannot fit its full column labels, Sanad shows each row as labeled fields instead. Report values appear literally, including Markdown punctuation in refs, paths, and messages. Control characters appear as visible escapes.

Proposed file diffs follow the report, with additions in green, deletions in red, and headers and hunks in cyan. Diff indentation, tabs, and line breaks are preserved; terminal control characters are shown as visible escapes. Diffs are not wrapped.

Redirected output uses plain tables without ANSI styling and an 80-column layout. Set `NO_COLOR=1` to disable styling in a terminal; `TERM=dumb` also disables it. `CLICOLOR=0` disables color unless overridden by `CLICOLOR_FORCE`. Redirection stays plain even when `CLICOLOR_FORCE` is set.

`markdown` produces the raw Markdown review summary for saving or publishing. JSON carries the complete structured result, including file diffs.

`config` and `version` have their own small outputs; they are not repository reports. See [CLI commands](../cli/) for their behavior.

## Report fields

| Field | Meaning |
| --- | --- |
| `version` | Report schema version; currently `1`. |
| `command`, `success` | Requested operation and whether it succeeded. |
| `backend_version`, `parser_version`, `lockfile_version` | Native components and lockfile schema used. |
| `native_support` | Recorded support confirmation, not an automatic entitlement result. |
| `verification` | `online`, `offline`, or `none`. |
| `coverage` | Total workflows, covered workflows, and uncovered paths. |
| `findings` | Errors and warnings, with codes and optional file paths. |
| `dependencies` | Locked graph entries, identities, commits, and workflow coverage. |
| `changes` | Before/after dependency graph changes. |
| `decisions` | Selected targets, pending candidates, reasons, and eligible dates. |
| `files` | Managed paths and proposed diffs. |
| `changed` | Whether repository changes were actually written. |

A preview can include `files` and `changes` while `changed` remains `false`. Use `changed`, not the presence of a diff, to decide whether the repository was modified.

## Verification levels

| Level | What it establishes |
| --- | --- |
| `online` | Native frozen verification, including source identity and reachability checks. |
| `offline` | Local workflow coverage and graph integrity. |
| `none` | No frozen verification result, as with status or an early operational error. |

A successful status command is not equivalent to a successful online check. No local result proves that GitHub runtime enforcement is enabled for a repository.

## Findings and decisions

Findings have a `code`, `severity`, and `message`; they may also identify a `file`. Error findings block the command. Warnings provide review context, such as an external reusable-workflow boundary or a policy change.

Update decisions explain `eligible`, `pending`, `unchanged`, or `frozen` candidates. Pending decisions can include an `eligible_at` timestamp. A plan containing pending candidates can succeed because waiting is a normal policy outcome.

## Exit codes

| Code | Meaning | Automation response |
| --- | --- | --- |
| `0` | Command succeeded. | Inspect any decisions or warnings before applying changes. |
| `1` | Blocking verification or policy findings. | Review the findings and fix or wait as appropriate. |
| `2` | Operational, parsing, or configuration error. | Correct the input, environment, or tool setup. |

Handle both the process exit code and the JSON `success` field. Do not assume that a report file means the command passed.

## Action reports

The [GitHub Action](../action/) writes JSON and Markdown files on the runner and exposes their paths as outputs. It also produces a job summary and annotations for findings. Preview runs intentionally report no changed files for staging, while retaining the full proposed diffs in JSON.

Remote reusable workflows appear as dependencies using their workflow path and
tracked ref. Their executable SHA is stored in the caller's YAML; associated
versions and numeric identities live in `.github/sanad/workflows.json`.

## Protection surfaces

The `protection` array (also shown in terminal and Markdown output) distinguishes
native action pins, inline SHA workflow calls, image digests, mutable containers,
workspace actions, runner environments, runtime tools, and shell commands. Each
entry includes its source location and a reason. A recorded native pin is not a
claim that online verification ran or that runtime enforcement is enabled.

Runtime dependencies and runner image contents are marked unknown or uninspected.
Sanad does not resolve shell scripts, `npx`, Dockerfile base images, or remote
action implementation downloads. An image digest covers that image, not code it
fetches while running. Coverage is an inventory of visible metadata, not a complete
software bill of materials for the executing workflow.

You can include a zizmor audit without adding another executable to Sanad:

```sh
zizmor --format sarif . > zizmor.sarif
sanad status --zizmor-report zizmor.sarif
sanad check --zizmor-report zizmor.sarif
```

The path must be inside the repository. Sanad imports SARIF 2.1.0 findings from
zizmor and preserves warning/error severity; errors block writes and checks.
The report must describe the same revision being reviewed. Sanad does not install
or execute the linter, or treat a clean imported report as proof of full coverage.
