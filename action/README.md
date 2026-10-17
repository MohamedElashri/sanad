# Sanad Action

Pin the Action to a full commit of a published 1.x release. Check out the repository
first. For observation-policy checks, fetch the base commit (`fetch-depth: 0`).

| Input | Default | Purpose |
| --- | --- | --- |
| `mode` | `check` | `check`, `status`, `plan`, `sync`, `update`, `setup` |
| `write` | `false` | Apply changes in `sync` or `update` |
| `base` | empty | Trusted base revision for adoption-policy checks |
| `config` | `.sanad.toml` | Repository-relative policy file |
| `working-directory` | `.` | Repository directory inside the workspace |
| `token` | `github.token` | Credentials for the repository host |

Outputs: `passed`, `changed`, `changed-files` (JSON), `updates`, `pending`,
`report-path`, `pr-body-path`, and `sanad-version`. Previews return an empty
`changed-files` output. Full proposed diffs remain in the JSON report.

The Action verifies an immutable Sanad release asset and checks the CLI version
matches its own. The native backend is bundled or checksum-pinned. Tokens are
masked; tenant tokens are not sent to the public release API.

Use `.github/workflows/update-pr.yml` for scheduled PRs, including observation-only
changes. PR checks need read permissions; the scheduled workflow needs contents
and pull-request writes. The CLI itself does not commit, push, or open PRs.

Development: `npm ci`, `npm test`, `npm run build`. `npm run test:integration`
requires `SANAD_ACTION_BINARY` and `SANAD_BACKEND` pointing to built/official tools.
