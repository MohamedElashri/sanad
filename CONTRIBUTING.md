# Contributing

## Development

Requires Go 1.26.3+ and Node.js 24+.

```sh
make check
npm ci --prefix action
npm test --prefix action
```

Markdown prose wraps at 80 columns. Format it with:

```sh
npx prettier@3.6.2 --write --ignore-path .gitignore '**/*.md'
```

Code, tables, links, and front matter can exceed the prose width.

## Repository dependency checks

We use Sanad v1.1.0 to verify our workflows, approve direct and transitive
source identities, and guard both published action entry points. Pull-request
checks attach JSON and Markdown reports with dependency and protection details.

Install Sanad 1.1+ and keep its matching backend on `PATH`, then run:

```sh
make sanad-check                 # online verification against origin/main
make sanad-local                 # local coverage without network access
make sanad-plan                  # preview eligible updates
pre-commit install               # optional; requires pre-commit
```

The local hook checks coverage; online CI enforces source approvals. Review and
merge approval changes before introducing dependencies from new repositories.
Sanad proposes Actions updates on Mondays at 03:00 UTC, or through a manual run
of the **Sanad updates** workflow. It uses the workflow-capable `SANAD_TOKEN`
repository secret and maintains one update PR. Dependabot handles Go and npm
dependencies.

The updater currently pins a reviewed workflow fix by commit. After the next
Sanad release, explicitly update this reusable-workflow pin to the release tag
to resume its automatic version updates.
