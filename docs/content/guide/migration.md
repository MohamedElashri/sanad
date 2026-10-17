+++
title = "Reset from 0.x"
description = "Replace legacy Sanad metadata with a native lockfile."
template = "page"
+++

Sanad 1.0 removes the old commands, configuration schema, Action inputs, and inline
SHA management. There is no migration importer or compatibility mode.

1. Remove `.github/sanad.lock.json` and the old `.sanad.toml`.
2. Remove `# sanad:` comments and choose the readable `uses:` refs you want.
3. Remove old Sanad automation and add the [current CI workflow](../ci/).
4. Follow the [quickstart](../getting-started/) and review every newly resolved
   commit before accepting the native baseline.

For Homebrew, replace the old formula with the cask:

```sh
brew uninstall --formula MohamedElashri/sanad/sanad
brew install --cask MohamedElashri/sanad/sanad
```

Native lockfile support must be confirmed for the repository. Actions that
generate local sub-actions, including `pypa/gh-action-pypi-publish@v1.14.2`, are
currently rejected by GitHub's producer. See [security and limitations](../../advanced/security-model/).
