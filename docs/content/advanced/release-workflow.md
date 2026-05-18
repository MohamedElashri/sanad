+++
title = "Release Workflow"
description = "How Sanad release artifacts and docs are published."
weight = 40
template = "page"
+++

Sanad uses GoReleaser for tagged releases.

Release publishing is triggered by tags that match `v*`:

```bash
git tag v0.1.0
git push origin v0.1.0
```

GoReleaser builds Linux, macOS, and Windows archives for amd64 and arm64, includes README, LICENSE, and `.sanad.toml.example`, and publishes SHA-256 checksums.

The `version` command is populated through linker metadata in release builds:

```bash
sanad version
```

## Documentation publishing

The docs site is built with Nida from `docs/` and deployed with GitHub Pages.

The Pages workflow:

1. Checks out the repository.
2. Sets up Go.
3. Installs a pinned Nida release.
4. Generates `docs/content/release-notes.md` from `CHANGELOG.md`.
5. Builds `docs/public`.
6. Uploads and deploys the Pages artifact.

Generated docs output is not committed.
