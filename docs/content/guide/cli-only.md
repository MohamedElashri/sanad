+++
title = "CLI-only integrations"
description = "Use release binaries with tokens, Make, and local hooks."
weight = 35
template = "page"
+++

Release archives contain two executables: Sanad and its checksum-pinned native
backend. They need no Node.js, Go installation, npm dependencies, or GitHub CLI
when you supply `GH_TOKEN` (or `GITHUB_TOKEN`). Git is used for repository discovery
and trusted-base comparisons. Pass `--root` and `--repo` for explicit identity.

```sh
export GH_TOKEN="<read-token>"
sanad init --root . --repo owner/repo --confirm-native
sanad init --root . --repo owner/repo --confirm-native --write
sanad check --base origin/main
```

For public repositories, use a fine-grained token limited to the reads you need;
private sources need Contents read permission. Local checking does not require
workflow write credentials. Shell history and secret handling follow your usual
token practices. Stored GitHub CLI credentials remain an optional alternative.
The Action is also optional; any CI runner can invoke the installed CLI.

Copy the targets in [examples/cli/Makefile](https://github.com/MohamedElashri/sanad/blob/main/examples/cli/Makefile)
into your Makefile. The online target uses a trusted base; the local target only
checks structure and coverage:

```sh
make sanad-check SANAD_BASE=origin/main
make sanad-local
```

The repository provides a system-language pre-commit hook. Set `rev` to the full
Sanad release commit you reviewed, and install Sanad and its matching backend on
your PATH before running pre-commit:

```yaml
repos:
  - repo: https://github.com/MohamedElashri/sanad
    rev: <full-reviewed-release-commit>
    hooks:
      - id: sanad-check
```

The hook invokes `sanad check --offline --coverage-only`. This explicit developer
mode skips adoption waiting periods and source approvals; it still validates local
coverage and recorded workflow pins. It cannot be combined with `--base`, and is
not a replacement for online CI checks. Plain `check --offline` continues to reject
repositories whose adoption policy requires online verification.

New releases publish SPDX inventories of the compiled CLI/backend in every archive
and production npm dependencies of the Action. Unknown package licenses remain
`NOASSERTION`; the inventories do not assert that all runtime downloads are known.
GitHub build-provenance attestations cover the archives, checksum manifest, and
inventory files before publication. For releases containing these artifacts:

```sh
gh attestation verify sanad_<version>_Linux_x86_64.tar.gz --repo MohamedElashri/sanad
```

Older releases do not acquire attestations retroactively. Token-only operation of
Sanad itself does not depend on this optional GitHub CLI verification command.

For Enterprise Cloud data-residency repositories, keep tenant and public reads
separate when running without GitHub CLI:

```sh
export GH_HOST="customer.ghe.com"
export GH_TOKEN="<tenant-read-token>"
export SANAD_GITHUB_COM_TOKEN="<public-github-com-read-token>"
sanad plan --repo customer.ghe.com/owner/repo
```

`SANAD_GITHUB_COM_TOKEN` is used only for public github.com sources while targeting
a tenant. It is passed to the pinned producer through its dedicated public-token
setting. Tenant credentials are never reused for public API calls. Without this
optional credential, public reads use stored github.com credentials or anonymous
access and its lower rate limit.
