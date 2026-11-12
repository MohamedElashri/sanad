<div align="center">
  <img src="docs/static/logo.svg" alt="Sanad" width="96">
  <h1>Sanad</h1>
  <p>Make native GitHub Actions lockfiles easy to adopt, review, and update.</p>
</div>

Sanad manages GitHub's native [`actions.lock`](https://docs.github.com/en/actions/concepts/security/workflow-dependency-lockfiles).
Pin direct and transitive actions, review updates, and enforce dependency policies
in CI.

[Documentation](https://melashri.net/sanad/) ·
[Releases](https://github.com/MohamedElashri/sanad/releases) ·
[Agent skill](skills/sanad/SKILL.md)

## Quick start

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

Commit the reported files. See [getting started](docs/content/guide/getting-started.md)
for setup details and [CI integration](docs/content/guide/ci.md) for PR checks.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and repository checks.

## License

[MIT](LICENSE).
