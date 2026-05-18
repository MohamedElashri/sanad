+++
title = "Sanad Docs"
description = "A focused CLI for pinning GitHub Actions workflow dependencies to immutable commit SHAs."
sort_by = "weight"
+++

Sanad scans GitHub Actions workflow files, resolves action refs through GitHub, and rewrites mutable `uses:` entries to full commit SHAs. It keeps the logical ref as metadata so future runs can keep tracking the intended update channel.

These docs are split by audience:

- Use the Guide when you want to install Sanad, pin workflows, and run it in CI.
- Use the Reference when you need exact command flags, config keys, and lockfile fields.
- Use Advanced when you want the security model, internals, or contribution workflow.
