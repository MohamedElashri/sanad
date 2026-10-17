+++
title = "Release notes"
description = "The 1.0 breaking redesign."
template = "page"
+++

1.0 replaces inline SHA management with GitHub’s native lockfile. New commands
are `init`, `sync`, `plan`, `update`, and `check`. Optional first-seen policy covers
transitive changes. Reports share one schema; exit codes are 0/1/2.

The old lockfile, comments, configuration schema, Action inputs, and commands are
removed. Reset old projects and initialize again. See the [quickstart](../guide/getting-started/).

[Historical changelog](https://github.com/MohamedElashri/sanad/blob/main/CHANGELOG.md).
