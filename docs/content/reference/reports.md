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

The default `table` format includes the review summary and proposed file diffs. `markdown` produces a review summary. JSON carries the complete structured result, including file diffs.

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
