# Forage

Discover package files, digests, and provenance from package indexes.

Forage queries standard package index APIs to retrieve filenames, checksums, and supply chain provenance attestations — without relying on index-specific APIs. Currently supports [PEP 503](https://peps.python.org/pep-0503/) Python indexes (PyPI, Pulp, devpi, Artifactory, etc.) and the [npm registry](https://docs.npmjs.com/cli/v10/using-npm/registry), with plans to expand to more ecosystems (Maven and others). Packages can be looked up by ecosystem (`forage python`, `forage npm`) or via a [Package URL](https://github.com/package-url/purl-spec) (`forage purl`).

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

Forage provides three subcommands:

#### `forage python` — look up a Python package

```
forage python [flags] <package> <version>
```

| Flag | Description | Default |
|---|---|---|
| `--index-url` | PEP 503 simple index URL | `https://pypi.org/simple/` |
| `--json` | Output JSON instead of human-readable text | `false` |
| `--fetch-provenance` | Fetch and inline provenance attestation data | `false` |

#### `forage npm` — look up an npm package

```
forage npm [flags] <package> <version>
```

| Flag | Description | Default |
|---|---|---|
| `--registry-url` | npm registry URL | `https://registry.npmjs.org` |
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

The index URL is derived from the purl's `repository_url` qualifier, falling back to the ecosystem default when absent. Supported purl types: `npm`, `pypi`.

### Go library

```go
import "github.com/lcarva/forage"

// Python (PEP 503)
result, err := forage.Lookup(ctx, "cryptography", "48.0.0", &forage.Options{
    IndexURL:        forage.DefaultIndexURL,
    FetchProvenance: true,
})

// npm
result, err := forage.NpmLookup(ctx, "express", "4.21.2", &forage.Options{
    RegistryURL:     forage.DefaultNpmRegistryURL,
    FetchProvenance: true,
})
// result.Files contains filename, digests (algorithm + hex value), and optionally provenance data
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

### Basic npm lookup

```
$ forage npm express 4.21.2

express 4.21.2

  express-4.21.2.tgz
    sha512: dbc1ea80c6409a28750b3b7d9f2eeaafa7a4daa75d17815c95b333c21091101e8e15f1fead7027b8d0b0a35ff016faed6e0b100dbe2449b5fd75ef6515bad79c
    sha1: cf250e48362174ead6cea4a566abef0162c1ec32
    provenance: (none)
```

### Scoped npm package

```
$ forage npm @babel/core 7.26.10

@babel/core 7.26.10

  core-7.26.10.tgz
    sha512: bccab26fb5c20cc3ef24514e693f64c6d891878d86c25644835fee220b59b214b96bff0e69db947c28bb9329ca81cdd3c3fe94a360fe75bf6a06db608619b1c1
    sha1: 5c876f83c8c4dcb233ee4b670c0606f2ac3000f9
    provenance: (none)
```

### npm lookup with provenance

```
$ forage npm --fetch-provenance sigstore 3.1.0

sigstore 3.1.0

  sigstore-3.1.0.tgz
    sha512: 669cd60051c816ac85139e9d5ea817fc3903459773fab45c8e8224fd1414e085f4c220afd65f12ed9ad70c770273eb9a7feea8ef0de1da5de0e86606e5328df5
    sha1: 08dc6c0c425263e9fdab85ffdb6477550e2c511d
    provenance: (none)
    attestations: 2
      - https://slsa.dev/provenance/v1
      - https://github.com/npm/attestation/tree/main/specs/publish/v0.1
    (use --json for full provenance data)
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

npm purl lookups work the same way. Scoped packages use the purl namespace:

```
$ forage purl pkg:npm/express@4.21.2
$ forage purl pkg:npm/%40babel/core@7.26.10
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
      "digests": [
        {
          "algorithm": "sha256",
          "value": "0c558d2cdffd8f4bbb30fc7134c74d2ca9a476f830bb053074498fbc86f41ed6"
        }
      ],
      "provenance_url": "https://pypi.org/integrity/cryptography/48.0.0/cryptography-48.0.0-cp311-abi3-macosx_10_9_universal2.whl/provenance",
      "provenance": {
        "publisher": {
          "kind": "GitHub",
          "repository": "pyca/cryptography",
          "workflow": "pypi-publish.yml"
        },
        "attestations": [
          {
            "mediaType": "application/vnd.pypi.attestation+json",
            "predicateType": "https://docs.pypi.org/attestations/publish/v1",
            "bundle": {"..."}
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
   - **Digests** — sha256 hash from the URL fragment (`#sha256=...`)
   - **Provenance URL** — from the `data-provenance` attribute ([PEP 740](https://peps.python.org/pep-0740/))
3. Filters to files matching the requested version
4. Optionally fetches each provenance URL to inline the attestation bundle

This approach is entirely standards-based and does not depend on any index-specific API (e.g. PyPI's JSON API or Pulp's REST API).

For npm packages, Forage uses the [npm registry API](https://github.com/npm/registry/blob/main/docs/responses/package-metadata.md):

1. Fetches the version metadata for the given package (`{registry}/{package}/{version}`)
2. Extracts from the `dist` object:
   - **Filename** — from the tarball URL
   - **Digests** — the `integrity` field (SRI format, e.g. `sha512-...`) is decoded to hex, and the `shasum` field (SHA-1) is included as-is
3. Optionally fetches provenance attestations from the registry's attestation endpoint (`/-/npm/v1/attestations/{package}@{version}`)

## Notes on provenance

Provenance attestation data is normalized into a common format across all ecosystems. Each attestation includes a `mediaType` (so downstream consumers know how to verify it), an optional `predicateType`, and the verbatim `bundle` from the source registry. The bundle is preserved as-is for signature verification — only the outer structure is normalized.

For Python/PyPI, the `data-provenance` attribute is defined by [PEP 740](https://peps.python.org/pep-0740/). Not all indexes support it — when absent, the provenance field will show `(none)` or `null`. PyPI attestation bundles use `application/vnd.pypi.attestation+json` as their media type.

For npm, attestations are fetched from the registry's attestation endpoint and use Sigstore bundles (`application/vnd.dev.sigstore.bundle+json`).

## Roadmap

Forage currently supports Python (PEP 503) indexes and the npm registry. Future ecosystem support is planned:

- **Maven** — query Maven Central / Sonatype for artifact checksums and signatures
- **Additional ecosystems** — contributions welcome
