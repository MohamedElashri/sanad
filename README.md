<div align="center">
  <img src="docs/static/logo.svg" alt="Sanad" width="96">
  <h1>Sanad</h1>
  <p>Make native GitHub Actions lockfiles easy to adopt, review, and update.</p>
</div>

Sanad manages GitHub's native
[`actions.lock`](https://docs.github.com/en/actions/concepts/security/workflow-dependency-lockfiles).
Pin direct and transitive actions, review updates, and enforce dependency
policies in CI.

[Documentation](https://melashri.net/sanad/) ·
[Releases](https://github.com/MohamedElashri/sanad/releases) ·
[Agent skill](skills/sanad/SKILL.md)

## CLI quick start

```sh
brew install --cask MohamedElashri/sanad/sanad
```

Linux, macOS, and Windows binaries are also available in
[release archives](https://github.com/MohamedElashri/sanad/releases/latest).
Keep the included `sanad` and `gh-actions-lock` executables together on `PATH`.

Your repository must support GitHub's native lockfiles. Authenticate with
`GH_TOKEN`, `GITHUB_TOKEN`, or `gh auth login`, then run:

```sh
sanad init --confirm-native          # preview the initial lockfile
sanad init --confirm-native --write  # apply the reviewed changes
sanad check                         # verify dependencies
```

Commit the reported files. See
[getting started](docs/content/guide/getting-started.md) for setup details and
[updating dependencies](docs/content/guide/updating.md) for ongoing maintenance.

## GitHub Action quick start

After initializing the lockfile, add `.github/workflows/sanad.yml`:

```yaml
name: Sanad
on: pull_request
permissions:
  contents: read
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          fetch-depth: 0
      - uses: MohamedElashri/sanad@v1.1.0
        with:
          mode: check
          base: ${{ github.event.pull_request.base.sha }}
```

Run `sanad sync --write` and commit the workflow with the updated lockfile. Make
the check required in repository settings. See
[CI setup](docs/content/guide/ci.md) for automation and the
[Action reference](docs/content/reference/action.md) for inputs and outputs.

For advanced use, see [configuration](docs/content/reference/configuration.md)
and the [security model](docs/content/advanced/security-model.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and repository
checks.

## License

[MIT](LICENSE).
