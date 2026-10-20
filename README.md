<div align="center">
  <img src="docs/static/logo.svg" alt="Sanad" width="96">
  <h1>Sanad</h1>
  <p>Make native GitHub Actions lockfiles easy to adopt, review, and update.</p>
</div>

Sanad manages GitHub's native [`actions.lock`](https://docs.github.com/en/actions/concepts/security/workflow-dependency-lockfiles).
It generates lockfiles, explains dependency changes, applies update policies, and
verifies them in CI. GitHub enforces the locked commits when workflows run.

Requires [native lockfile support](https://docs.github.com/en/actions/how-tos/secure-your-work/lock-workflow-dependencies)
for your repository. Native lockfiles are in GitHub's public preview.

## CLI quickstart

Install with Homebrew:

```sh
brew install --cask MohamedElashri/sanad/sanad
```

Or download a [v1.0.0 archive](https://github.com/MohamedElashri/sanad/releases/tag/v1.0.0)
for Linux, macOS, or Windows on amd64/arm64. Keep the included `sanad` and
`gh-actions-lock` executables together on your `PATH`.

In your repository, authenticate and review the initial lockfile:

```sh
gh auth login
sanad init --confirm-native
sanad init --confirm-native --write
sanad check
```

`--confirm-native` records that you verified GitHub supports enforcement for this
repository. Review the preview before writing, then commit
`.github/workflows/actions.lock` and `.sanad.toml`. Authentication also accepts
`GH_TOKEN` or `GITHUB_TOKEN`.

## Action quickstart

Add `.github/workflows/sanad.yml` to check pull requests:

```yaml
name: Sanad
on: pull_request
permissions:
  contents: read
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7.0.1
        with:
          fetch-depth: 0
      - uses: MohamedElashri/sanad@v1.0.0
        with:
          mode: check
          base: ${{ github.event.pull_request.base.sha }}
```

Run `sanad sync --write` after adding the workflow and commit the updated lockfile.
Make the Sanad check required in repository settings. The Action installs its
matching CLI release and verifies its checksum and immutability.

See [CI setup](docs/content/guide/ci.md) for generated workflows and scheduled
update PRs, or the [Action reference](docs/content/reference/action.md) for inputs and outputs.

## Everyday use

```sh
sanad sync                  # preview reconciliation after workflow edits
sanad sync --write           # apply it without advancing existing pins
sanad plan                  # review available stable version updates
sanad update --write         # apply eligible updates
sanad check                  # online frozen verification
sanad check --offline        # local coverage and graph checks only
```

Repository changes require `--write`. Updates stay within the current major
version by default. Optional constraints and waiting periods cover direct and
transitive dependencies; see [configuration reference](docs/content/reference/configuration.md).

## Coding agents

Install the reusable Sanad skill for Codex, Claude Code, Cursor, GitHub Copilot,
Antigravity, or OpenCode by copying the skill directory into your agent's skill
location. For example, from a Sanad checkout, install it for your user in Codex:

```sh
mkdir -p "$HOME/.agents/skills"
cp -R skills/sanad "$HOME/.agents/skills/"
```

The skill covers onboarding, workflow reconciliation, scoped updates, and
verification. It requires Sanad 1.x on the agent's `PATH`. See the
[agent guide](docs/content/guide/agents.md) for each agent's installation paths
and example requests, or read [SKILL.md](skills/sanad/SKILL.md).

[Documentation](https://melashri.net/sanad/) ·
[Security and limitations](docs/content/advanced/security-model.md) ·
[Release notes](docs/content/release-notes.md) · [MIT license](LICENSE)

## Development

Requires Go 1.26.3+ and Node.js 24+.

```sh
make check
npm ci --prefix action
npm test --prefix action
```
