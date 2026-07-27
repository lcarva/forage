# Forage

Discover package files, digests, and provenance from package indexes.

Forage queries standard package index APIs to retrieve filenames, checksums, and supply chain provenance attestations — without relying on index-specific APIs. Currently supports [PEP 503](https://peps.python.org/pep-0503/) Python indexes (PyPI, Pulp, devpi, Artifactory, etc.), with plans to expand to other ecosystems (npm, Maven, and more).

## Requirements

- Go 1.25 or later

## Install

```
go install github.com/lcarva/forage/cmd/forage@latest
```

### Build from source

```
git clone https://github.com/lcarva/forage.git
cd forage
make build
```

## Usage

### CLI

```
forage [flags] <package> <version>
```

| Flag | Description | Default |
|---|---|---|
| `--index-url` | PEP 503 simple index URL | `https://pypi.org/simple/` |
| `--json` | Output JSON instead of human-readable text | `false` |
| `--fetch-provenance` | Fetch and inline provenance attestation data | `false` |

### Go library

```go
import "github.com/lcarva/forage"

result, err := forage.Lookup(ctx, "requests", "2.32.2", &forage.Options{
    IndexURL:        forage.DefaultIndexURL,
    FetchProvenance: true,
})
// result.Files contains filename, sha256, provenance URL, and optionally provenance data
```

## Examples

### Basic lookup on PyPI

```
$ forage requests 2.32.2

requests 2.32.2

  requests-2.32.2-py3-none-any.whl
    sha256: fc06670dd0ed212426dfeb94fc1b983d917c4f9847c863f313c9dfaaffb7c23c
    provenance: (none)

  requests-2.32.2.tar.gz
    sha256: dd951ff5ecf3e3b3aa26b40703ba77495dab41da839ae72ef3c8e5d8e2433289
    provenance: (none)
```

### Custom index with provenance

```
$ forage --index-url https://packages.redhat.com/trusted-libraries/python/ requests 2.32.5

requests 2.32.5

  requests-2.32.5-0-py3-none-any.whl
    sha256: 2228e951928fab47a74f7be4f533eaf1f389e08a4f8dac3da76e2ec0552f9cf5
    provenance: https://packages.redhat.com/api/pypi/.../provenance/

  requests-2.32.5.tar.gz
    sha256: 8477bb641a004933eaa481124a3917dd3b384256ad4eff3084afd53d6503a8c0
    provenance: https://packages.redhat.com/api/pypi/.../provenance/
```

### JSON output with fetched provenance

```
$ forage --index-url https://packages.redhat.com/trusted-libraries/python/ \
    --json --fetch-provenance requests 2.32.5

{
  "package": "requests",
  "version": "2.32.5",
  "files": [
    {
      "filename": "requests-2.32.5-0-py3-none-any.whl",
      "sha256": "2228e951928fab47a74f7be4f533eaf1f389e08a4f8dac3da76e2ec0552f9cf5",
      "provenance_url": "https://...",
      "provenance": {
        "version": 1,
        "attestation_bundles": [...]
      }
    }
  ]
}
```

## Development

Run the full CI suite locally:

```
make ci
```

Individual targets are also available:

| Target | Description |
|---|---|
| `make fmt` | Check formatting (`gofmt`) |
| `make vet` | Run `go vet` |
| `make test` | Run tests |
| `make build` | Build all packages |
| `make ci` | Run `fmt`, `vet`, and `test` |

## How it works

For Python indexes, Forage uses the [PEP 503 Simple Repository API](https://peps.python.org/pep-0503/):

1. Fetches the simple index page for the given package (`{index_url}/{package}/`)
2. Parses the HTML to extract from each `<a>` tag:
   - **Filename** — link text
   - **sha256 digest** — from the URL fragment (`#sha256=...`)
   - **Provenance URL** — from the `data-provenance` attribute ([PEP 740](https://peps.python.org/pep-0740/))
3. Filters to files matching the requested version
4. Optionally fetches each provenance URL to inline the attestation bundle

This approach is entirely standards-based and does not depend on any index-specific API (e.g. PyPI's JSON API or Pulp's REST API).

## Notes on provenance

The `data-provenance` attribute is defined by [PEP 740](https://peps.python.org/pep-0740/). Not all indexes support it — when absent, the provenance field will show `(none)` or `null`. The attestation bundles typically contain [in-toto](https://in-toto.io/) statements with [SLSA](https://slsa.dev/) provenance predicates.

## Roadmap

Forage currently supports Python (PEP 503) indexes. Future ecosystem support is planned:

- **npm** — query the npm registry for package tarballs, digests, and provenance
- **Maven** — query Maven Central / Sonatype for artifact checksums and signatures
- **Additional ecosystems** — contributions welcome
