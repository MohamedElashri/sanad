# Sanad Action

Initialize the repository with `sanad init --confirm-native --write`, then add a
check to your pull-request workflow:

```yaml
- uses: actions/checkout@v7.0.1
  with:
    fetch-depth: 0
- uses: MohamedElashri/sanad@v1.0.0
  with:
    mode: check
    base: ${{ github.event.pull_request.base.sha }}
```

Run `sanad sync --write` and commit the updated lockfile after adding the workflow.
You can also pin the Action to the release's full commit. The trusted `base` and
full checkout history are required when enforcing observation policy.

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

Generate scheduled PR workflows with `sanad init --automation --write`, including
observation-only changes. PR checks need read permissions; the scheduled workflow needs contents
and pull-request writes. The CLI itself does not commit, push, or open PRs.

Development: `npm ci`, `npm test`, `npm run build`. `npm run test:integration`
requires `SANAD_ACTION_BINARY` and `SANAD_BACKEND` pointing to built/official tools.

[Full Action reference](https://melashri.net/sanad/reference/action/) · [CI setup](https://melashri.net/sanad/guide/ci/)
