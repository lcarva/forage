# Forage

Discover package files, digests, and provenance from package indexes.

Forage queries standard package index APIs to retrieve filenames, checksums, and supply chain provenance attestations — without relying on index-specific APIs. Currently supports [PEP 503](https://peps.python.org/pep-0503/) Python indexes (PyPI, Pulp, devpi, Artifactory, etc.), with plans to expand to other ecosystems (npm, Maven, and more).

## Requirements

- Python 3.x
- [requests](https://pypi.org/project/requests/)

```
pip install requests
```

## Usage

```
python forage.py [-h] [--index-url INDEX_URL] [--json] [--fetch-provenance] package version
```

### Arguments

| Argument | Description |
|---|---|
| `package` | Package name |
| `version` | Package version |
| `--index-url` | PEP 503 simple index URL (default: `https://pypi.org/simple/`) |
| `--json` | Output JSON instead of human-readable text |
| `--fetch-provenance` | Fetch and inline provenance attestation data |

## Examples

### Basic lookup on PyPI

```
$ python forage.py requests 2.32.2

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
$ python forage.py requests 2.32.5 \
    --index-url https://packages.redhat.com/trusted-libraries/python/

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
$ python forage.py requests 2.32.5 \
    --index-url https://packages.redhat.com/trusted-libraries/python/ \
    --json --fetch-provenance

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
