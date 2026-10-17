+++
title = "Release notes"
description = "Sanad 1.0.0."
template = "page"
+++

Sanad 1.0 manages GitHub's native Actions lockfile. Use `init` to onboard, `sync`
to reconcile workflow edits, `plan` and `update` for stable version changes, and
`check` for frozen verification. Optional first-seen policy covers transitive
changes and uses trusted base observations in CI.

Release archives bundle the verified official backend for Linux, macOS, and
Windows on amd64/arm64. Homebrew installs a cask. The Action installs its matching
immutable release and provides one structured report format.

For older installations, see [resetting from 0.x](../guide/migration/).
The native producer rejects actions with generated local sub-actions, including
`pypa/gh-action-pypi-publish@v1.14.2`; see [limitations](../advanced/security-model/).

[Full changelog](https://github.com/MohamedElashri/sanad/blob/main/CHANGELOG.md).
