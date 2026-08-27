# Forage

## Build & Test

```bash
make check     # Run everything: ci, integration, and zizmor
make ci        # Run fmt, vet, and test
make test      # Run tests only
make fmt       # Check formatting (gofmt)
make vet       # Run go vet
make zizmor    # Lint GitHub Actions workflows with zizmor (requires uv)
make build     # Build binary to bin/forage
```

## Rules

- When adding or modifying user-facing features (new CLI subcommands, new ecosystem
  support, new flags, library API changes), check if README.md needs updating and include
  those changes in the same PR.
