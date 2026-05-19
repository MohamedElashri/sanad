+++
title = "Development"
description = "Local development commands and contribution notes."
weight = 30
template = "page"
+++

Common commands:

```bash
make build
make test
make lint
make check
```

Race tests:

```bash
make race
```

Benchmarks:

```bash
make bench
```

Build the documentation site:

```bash
make docs-build
```

Serve the documentation site with Nida:

```bash
make docs-serve
```

Set `NIDA` when the binary is not on `PATH`:

```bash
NIDA=/path/to/nida make docs-build
```

## Docs layout

Sanad's Nida site lives under `docs/`.

```text
docs/
  config.toml
  content/
  templates/
  static/
  public/
```

`docs/content/release-notes.md` is generated from `CHANGELOG.md` by `scripts/generate_release_notes.go`.
