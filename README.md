# Forage

Discover package files, digests, and provenance from package indexes.

Forage queries standard package index APIs to retrieve filenames, checksums, and supply chain provenance attestations — without relying on index-specific APIs. Currently supports [PEP 503](https://peps.python.org/pep-0503/) Python indexes (PyPI, Pulp, devpi, Artifactory, etc.), with plans to expand to other ecosystems (npm, Maven, and more). Packages can be looked up by ecosystem (`forage python`) or via a [Package URL](https://github.com/package-url/purl-spec) (`forage purl`).

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

Forage provides two subcommands:

#### `forage python` — look up by ecosystem

```
forage python [flags] <package> <version>
```

| Flag | Description | Default |
|---|---|---|
| `--index-url` | PEP 503 simple index URL | `https://pypi.org/simple/` |
| `--json` | Output JSON instead of human-readable text | `false` |
| `--fetch-provenance` | Fetch and inline provenance attestation data | `false` |

#### `forage purl` — look up by Package URL

```
forage purl [flags] <package-url>
```

| Flag | Description | Default |
|---|---|---|
| `--json` | Output JSON instead of human-readable text | `false` |
| `--fetch-provenance` | Fetch and inline provenance attestation data | `false` |

The index URL is derived from the purl's `repository_url` qualifier, falling back to `https://pypi.org/simple/` when absent. Supported purl types: `pypi`.

### Go library

```go
import "github.com/lcarva/forage"

result, err := forage.Lookup(ctx, "cryptography", "48.0.0", &forage.Options{
    IndexURL:        forage.DefaultIndexURL,
    FetchProvenance: true,
})
// result.Files contains filename, sha256, provenance URL, and optionally provenance data
```

## Examples

### Basic lookup on PyPI

```
$ forage python cryptography 48.0.0

cryptography 48.0.0

  cryptography-48.0.0-cp311-abi3-macosx_10_9_universal2.whl
    sha256: 0c558d2cdffd8f4bbb30fc7134c74d2ca9a476f830bb053074498fbc86f41ed6
    provenance: https://pypi.org/integrity/cryptography/48.0.0/cryptography-48.0.0-cp311-abi3-macosx_10_9_universal2.whl/provenance

  cryptography-48.0.0-cp311-abi3-manylinux2014_aarch64.manylinux_2_17_aarch64.whl
    sha256: f5333311663ea94f75dd408665686aaf426563556bb5283554a3539177e03b8c
    provenance: https://pypi.org/integrity/cryptography/48.0.0/cryptography-48.0.0-cp311-abi3-manylinux2014_aarch64.manylinux_2_17_aarch64.whl/provenance

  ... (additional platform-specific wheels omitted)

  cryptography-48.0.0.tar.gz
    sha256: 5c3932f4436d1cccb036cb0eaef46e6e2db91035166f1ad6505c3c9d5a635920
    provenance: https://pypi.org/integrity/cryptography/48.0.0/cryptography-48.0.0.tar.gz/provenance
```

### Custom index with provenance

```
$ forage python --index-url https://packages.redhat.com/trusted-libraries/python/ cryptography 48.0.0

cryptography 48.0.0

  cryptography-48.0.0-0-cp312-abi3-manylinux_2_28_x86_64.whl
    sha256: 8bff3de351be0bcbb36d620376de068f2076d39906a9cff380a3de6523581de2
    provenance: https://packages.redhat.com/api/pypi/.../provenance/

  cryptography-48.0.0.tar.gz
    sha256: b69f619d07bdd1b98302fcb26332232b7b5d0bb17f73d92b53d495f6d2fa196d
    provenance: https://packages.redhat.com/api/pypi/.../provenance/
```

### Lookup via Package URL

```
$ forage purl pkg:pypi/requests@2.32.2

requests 2.32.2

  requests-2.32.2-py3-none-any.whl
    sha256: fc06670dd0ed212426dfeb94fc1b983d917c4f9847c863f313c9dfaaffb7c23c
    provenance: (none)

  requests-2.32.2.tar.gz
    sha256: dd951ff5ecf3e3b3aa26b40703ba77495dab41da839ae72ef3c8e5d8e2433289
    provenance: (none)
```

Use the `repository_url` qualifier to target a custom index:

```
$ forage purl "pkg:pypi/requests@2.32.5?repository_url=https://packages.redhat.com/trusted-libraries/python/"
```

### JSON output with fetched provenance

```
$ forage python --json --fetch-provenance cryptography 48.0.0

{
  "package": "cryptography",
  "version": "48.0.0",
  "files": [
    {
      "filename": "cryptography-48.0.0-cp311-abi3-macosx_10_9_universal2.whl",
      "sha256": "0c558d2cdffd8f4bbb30fc7134c74d2ca9a476f830bb053074498fbc86f41ed6",
      "provenance_url": "https://pypi.org/integrity/cryptography/48.0.0/cryptography-48.0.0-cp311-abi3-macosx_10_9_universal2.whl/provenance",
      "provenance": {
        "attestation_bundles": [
          {
            "attestations": ["..."]
          }
        ]
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
