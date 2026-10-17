<div align="center">
  <img src="docs/static/logo.svg" alt="Sanad" width="96">
  <h1>Sanad</h1>
  <p>Make native GitHub Actions lockfiles easy to adopt, review, and update.</p>
</div>

Sanad builds on GitHub's [`actions.lock`](https://docs.github.com/en/actions/concepts/security/workflow-dependency-lockfiles).
GitHub resolves and enforces execution pins. Sanad handles onboarding, update
selection, optional waiting periods, and readable review reports.

**This is the breaking 1.0 development branch.** Native lockfiles are currently
in GitHub's public preview; [check repository eligibility](https://docs.github.com/en/actions/how-tos/secure-your-work/lock-workflow-dependencies)
before enabling them. Sanad does not provide an inline SHA fallback.

## Start

Build this checkout with Go 1.26.3 or newer:

```sh
make build
bin/sanad status
bin/sanad init --confirm-native
bin/sanad init --confirm-native --write
```

`--confirm-native` records your confirmation that GitHub supports native lockfile
enforcement for this repository. It is not an entitlement check. Initial `init`
accepts the currently referenced code as the baseline; review the preview first.
Authenticate with `gh auth login`, `GH_TOKEN`, or `GITHUB_TOKEN`.

Release archives include the exact official backend. Source installations fetch
`gh-actions-lock v0.2.0` with pinned SHA-256 checksums on first online use.
Use `sanad setup` to preinstall it, or set `SANAD_BACKEND` to a verified official
binary. Linux, macOS, and Windows on amd64/arm64 are supported.

## Daily use

```sh
sanad sync                  # preview lockfile reconciliation after workflow edits
sanad sync --write           # preserve existing pins; add/remove dependency coverage
sanad plan                  # explain available stable version updates
sanad update --write         # apply eligible updates
sanad check                  # online frozen verification
sanad check --offline        # local coverage only; cached backend required
sanad check --base origin/main
```

Every repository write requires `--write`. Previews generate candidates in an
isolated copy. Branch and partial-version refs stay frozen; explicit version changes use:

```sh
sanad update --action actions/checkout --to v7.0.1 --write
sanad plan --level major
```

Scoped mutable-ref refresh is intentionally unavailable: the upstream CLI's
broad relock operation can move unrelated dependencies.

## Policy

`.sanad.toml` is optional except for the native-support confirmation:

```toml
version = 1

[native]
enabled = true

[update]
level = "minor"

# Optional: wait after each identity/ref/commit is first observed.
[observation]
minimum_age = "7d"

[actions."actions/checkout"]
constraint = ">= 6, < 8"
```

Updates select the highest eligible stable release. Per-action constraints replace
the global version limit. Dependabot's `github-actions.cooldown.default-days`
also applies; unsupported cooldown keys produce an error.

First-seen policy covers changed transitive dependencies too. Pending candidates
are recorded in `.github/sanad/state.json` by `update --write`, even when no
versions change. Commit those observations to the trusted base before expecting
a later update to pass `check --base`. A PR cannot supply its own maturity.
Observation-only PRs need merging, just like dependency update PRs.

## CI

After publishing a 1.x release, generate checking and scheduled-update workflows:

```sh
sanad init --automation --action-ref <SANAD_1_X_COMMIT> --write
```

Or run the Action directly, pinned to the full commit of that release:

```yaml
- uses: MohamedElashri/sanad@<SANAD_1_X_COMMIT>
  with:
    mode: check
    base: ${{ github.event.pull_request.base.sha }}
```

Check out the repository with `fetch-depth: 0` for base comparisons. Make the check
required in repository settings. Choose Dependabot or Sanad's scheduled updater
for version updates. See [CI](docs/content/guide/ci.md) and the
[Action reference](action/README.md).

## Reset from 0.x

Remove `.github/sanad.lock.json` and the old `.sanad.toml`. Remove `# sanad:`
comments, choose the readable `uses:` refs you want, then run `sanad init`.
Review all newly resolved commits. There is no migration command or legacy mode.

## Development

```sh
make check                  # tests, vet, staticcheck, build
make race
make test-native            # download and test the official backend
npm ci --prefix action
npm test --prefix action
npm run build --prefix action
make docs-build
```

Code lives in `internal/sanad` (native integration and policy) and `internal/cli`
(commands). [CLI reference](docs/content/reference/cli.md) ·
[Security and design](docs/content/advanced/security-model.md) · [MIT license](LICENSE).
