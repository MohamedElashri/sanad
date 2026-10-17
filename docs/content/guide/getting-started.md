+++
title = "Getting started"
description = "Adopt a native lockfile."
template = "page"
+++

Build with `make build`, then authenticate with `gh auth login`.

```sh
bin/sanad status
bin/sanad init --confirm-native
bin/sanad init --confirm-native --write
```

First [check GitHub eligibility](https://docs.github.com/en/actions/how-tos/secure-your-work/lock-workflow-dependencies).
`--confirm-native` records your confirmation; Sanad cannot detect entitlement.
Review the baseline, including transitive dependencies, before writing.

GitHub owns `.github/workflows/actions.lock`. Sanad never writes its own execution
lockfile. Run `sanad sync --write` after workflow edits, `sanad plan` to inspect
updates, and `sanad update --write` to apply eligible versions.

The first online command installs a checksum-pinned official backend if it is not
bundled. `sanad setup` installs it ahead of time. `SANAD_BACKEND` can point to the
exact official binary for offline environments. `check --offline` checks coverage
only, using the cached binary.

From 0.x, remove `.github/sanad.lock.json`, old configuration, and `# sanad:`
comments; choose desired readable refs and initialize again. Old commands and
metadata are unsupported.
