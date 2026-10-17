+++
title = "Reset from 0.x"
description = "Replace legacy Sanad metadata with a reviewed native baseline."
weight = 90
template = "page"
+++

Sanad 1.0 removes the old commands, configuration schema, Action inputs, and inline SHA management. There is no migration importer or compatibility mode.

## Reset the repository

1. Remove `.github/sanad.lock.json` and the old `.sanad.toml`.
2. Remove `# sanad:` comments and choose the readable `uses:` refs you want.
3. Remove old Sanad automation and add the [current CI workflow](../ci/).
4. Follow the [getting-started guide](../getting-started/), reviewing every newly resolved commit before accepting the native baseline.

The new native lockfile is `.github/workflows/actions.lock`. Commit it together with the new configuration and every reported workflow or local-action change.

## Replace the Homebrew formula

```sh
brew uninstall --formula MohamedElashri/sanad/sanad
brew install --cask MohamedElashri/sanad/sanad
```

## Review native compatibility

Native lockfile enforcement must be supported by your repository. The producer currently rejects actions that generate local sub-actions, including `pypa/gh-action-pypi-publish@v1.14.2`. Retain existing protection for affected workflows until you have a supported setup.

See [Security and design](../../advanced/security-model/) for the coverage boundaries and [Troubleshooting](../../advanced/troubleshooting/) for setup and verification failures.
