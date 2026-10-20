+++
title = "Coding agents"
description = "Teach coding agents to adopt, update, and verify Sanad lockfiles."
weight = 50
template = "page"
+++

Sanad includes a reusable [agent skill](https://github.com/MohamedElashri/sanad/tree/main/skills/sanad)
for Codex, Claude Code, Cursor, GitHub Copilot, Antigravity, and OpenCode. It teaches
agents to choose between `init`, `sync`, `plan`, `update`, and `check`, review
structured reports, respect update policies, and verify the resulting changes.
The same skill works across these tools. Its supporting references cover policy
configuration, CI automation, host/authentication setup, and troubleshooting.

## Install the skill

With Node.js 22.20 or later, run the optional
[Skills CLI](https://github.com/vercel-labs/skills) from the repository where you
want to use Sanad:

```sh
npx skills add MohamedElashri/sanad --skill sanad
```

Choose your coding agents when prompted. Installation defaults to the current
project; add `-g` for a personal installation shared across projects. The skill
provides instructions; install [Sanad 1.x](../getting-started/) separately and
make it available on the agent's `PATH`.

To select Antigravity CLI directly:

```sh
npx skills add MohamedElashri/sanad --skill sanad --agent antigravity-cli
```

To install from a local Sanad checkout before publishing your changes:

```sh
npx skills add /path/to/sanad --skill sanad
```

## Install without Node.js

Copy the entire `skills/sanad` directory, including `references/`, from a Sanad
source checkout into a skill directory your agent discovers. Paths in the project
column are relative to the repository where you want to use Sanad.

| Agent | Project directory | Personal directory | Documentation |
| --- | --- | --- | --- |
| Codex | `.agents/skills/sanad/` | `~/.agents/skills/sanad/` | [Skills](https://developers.openai.com/codex/skills/) |
| Claude Code | `.claude/skills/sanad/` | `~/.claude/skills/sanad/` | [Skills](https://code.claude.com/docs/en/skills) |
| Cursor | `.cursor/skills/sanad/` | `~/.cursor/skills/sanad/` | [Skills](https://cursor.com/help/customization/skills) |
| GitHub Copilot | `.github/skills/sanad/` | `~/.copilot/skills/sanad/` | [Skills](https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/customize-cloud-agent/add-skills) |
| Antigravity CLI | `.agents/skills/sanad/` | `~/.gemini/antigravity-cli/skills/sanad/` | [Skills](https://antigravity.google/docs/skills#cli-skill-locations) |
| Antigravity App / IDE | `.agents/skills/sanad/` | `~/.gemini/config/skills/sanad/` | [Skills](https://antigravity.google/docs/skills) |
| OpenCode | `.opencode/skills/sanad/` | `~/.config/opencode/skills/sanad/` | [Skills](https://opencode.ai/docs/skills/) |

For example, from your target repository, install for Codex or Antigravity:

```sh
mkdir -p .agents/skills
cp -R /path/to/sanad/skills/sanad .agents/skills/
```

Replace `/path/to/sanad` with your Sanad source checkout. For Claude Code, use
`.claude/skills` instead. For a personal Codex installation, use
`"$HOME/.agents/skills"` as the destination. The installed file should be
`SKILL_DIRECTORY/sanad/SKILL.md`. Start a new agent session after installation
if the skill does not appear. Commit project skills to share them with your team.

### Migrating from Gemini CLI

Google ended Gemini CLI consumer access on June 18, 2026 and directs those users
to Antigravity. See [Google's deprecation notice](https://developers.google.com/gemini-code-assist/docs/deprecations/code-assist-individuals)
for the affected accounts and separate enterprise schedule. This guide uses
Antigravity for new installations.

For existing skills, copy the `sanad` folder from `.gemini/skills/` to
`.agents/skills/` in your project. Antigravity CLI's personal skill directory is
`~/.gemini/antigravity-cli/skills/`; the App and IDE use
`~/.gemini/config/skills/`. These are distinct from Gemini CLI's old
`~/.gemini/skills/` location. See Google's
[migration guide](https://antigravity.google/docs/cli/gcli-migration/).

## Ask the agent to use Sanad

The skill's description lets agents select it for relevant requests. You can
also select it explicitly with `$sanad` in Codex or `/sanad` in Claude Code and
Antigravity CLI.
Example requests:

- "Use Sanad to reconcile the lockfile after my workflow edits, then verify it."
- "Use Sanad to preview patch updates for actions/checkout without writing files."
- "Use Sanad to update actions/checkout within our existing policy."
- "Explain why Sanad reports pending dependencies and when they become eligible."

For first-time adoption, confirm that the repository supports GitHub's native
lockfile enforcement before asking the agent to initialize it. Existing support
confirmation and repository policy remain part of the workflow. Preview requests
stay read-only; requests to apply changes use `--write` after the agent reviews
the preview.

Agents use `--format json` to read findings, decisions, and proposed file diffs.
They finish with online verification when available, passing a trusted `--base`
when first-seen policy is enabled or when evaluating adoption policy for a pull
request. They identify verification limitations when online checks cannot run;
offline checks cannot enforce adoption policy. See [reports](../../reference/reports/)
for the schema and [updating dependencies](../updating/) for the underlying workflow.
