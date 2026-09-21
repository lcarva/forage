# Forage

Forage is a Go library and CLI for discovering package files, digests, and
provenance from package indexes. It requires Go 1.25 or later.

## Build & Test

```bash
make ci          # Run formatting, vet, and unit tests
make test        # Run unit tests only
make fmt         # Check formatting with gofmt
make vet         # Run go vet
make build       # Build bin/forage
make integration # Run network-dependent integration tests; uses uvx and cosign
make zizmor      # Lint GitHub Actions workflows; requires uv
make check       # Run ci, integration, and zizmor
```

Run `make ci` after ordinary code changes. Run `make check` when the required
external tools and network access are available.

## Rules

- Add or update tests for behavior changes and bug fixes.
- For user-facing changes—such as CLI subcommands, flags, ecosystem support,
  output, or public library APIs—update README.md and any relevant files under
  `docs/` in the same change.
- Keep unit tests self-contained; use local HTTP test servers rather than live
  package indexes. Live-service checks belong in integration tests.
