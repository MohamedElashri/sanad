+++
title = "Sanad Docs"
description = "Pin and update GitHub Actions dependencies to immutable commit SHAs."
sort_by = "weight"
+++

Sanad pins and updates GitHub Actions dependencies by resolving action refs through GitHub and rewriting mutable `uses:` entries to full commit SHAs. It keeps the logical ref as metadata so future runs can keep tracking the intended update channel.

These docs are split by audience:

- Use the Guide when you want to install Sanad, pin workflows, and run it in CI.
- Use the Reference when you need exact command flags, config keys, and lockfile fields.
- Use Advanced when you want the security model, internals, or contribution workflow.
