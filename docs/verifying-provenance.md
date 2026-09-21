# Verifying Python provenance

Forage preserves [PEP 740](https://peps.python.org/pep-0740/) attestation objects
verbatim (`.provenance.attestations[].bundle`), but it does not verify them —
verification is an out-of-band step performed by the consumer. The right tool and
the right flags depend on the *trust model* used to sign the attestation, which is
not always the same as the one PyPI uses.

For the common case — a package from `pypi.org` — see
[Verifying attestations](../README.md#verifying-attestations) in the README, which
uses [`pypi-attestations`](https://github.com/trailofbits/pypi-attestations). This
document covers what changes when you verify against a **custom Python index**, a
**different Sigstore instance**, or a **long-lived signing key**.

## One format, three trust models

Every PEP 740 attestation object has the same shape, regardless of who signed it:

```
bundle
├── version
├── envelope
│   ├── statement    # base64 in-toto statement (the provenance predicate)
│   └── signature    # base64 DSSE signature
└── verification_material   # ← the trust model lives here
```

What differs is `verification_material`, i.e. *how the signature is trusted*:

| Trust model | `verification_material` | Verify against | Tooling |
|---|---|---|---|
| Keyless, **public** Sigstore | `certificate` (Fulcio) + `transparency_entries` (Rekor) | a workflow **identity** | `pypi-attestations` (easy path) or `cosign` |
| Keyless, **private** Sigstore | `certificate` + `transparency_entries`, but chaining to a private CA/log | a workflow **identity**, against your own instance | `cosign` (initialized against your instance) |
| **Long-lived key** | a public key, optionally with `transparency_entries` (Rekor) | a known **public key** | `cosign --key` |

`pypi-attestations` (and the `sigstore-python` CLI it builds on) is hardwired to
the **public** Sigstore instance and to certificate identities — it has no way to
point at a different Sigstore instance, and no path to verify a bare key. So the
first row is the only one it handles. The other two rows need `cosign`.

`cosign` spans all three rows, so the rest of this document uses it.

## The universal recipe: reshape to a Sigstore bundle

`cosign verify-blob-attestation --bundle` consumes a **Sigstore bundle**, not a
PEP 740 object, so the first step in every case is a small `jq` reshape. The
Sigstore bundle's `verificationMaterial` is a *oneof*, so the reshape differs only
in that field; the `dsseEnvelope` is identical everywhere.

We also need the digest and digest algorithm for verification. This avoids having to download the
artifact itself. As such, save the output of forage into a variable so we can extract all the
required values accordingly.

```bash
INFO="$(
    forage python --index-url "$INDEX_URL" --json --fetch-provenance "$PKG" "$VERSION" | \
    jq --arg artifact "${ARTIFACT}" \
    '.files[] | select(.filename == $artifact)')"
```

Get the PEP 740 object first:

```bash
<<< "${INFO}" jq '.provenance.attestations[0].bundle' > pep740.json
```

Then, the get the artifact's digest and digest algorithm:

```bash
DIGEST="$(<<< "${INFO}" jq -r '.digests[0].value')"
DIGESTALG="$(<<< "${INFO}" jq -r '.digests[0].algorithm')"
```

The digest must be raw hexadecimal: pass `5c39...`, not `sha256:5c39...`.
Both `--digest` and `--digestAlg` are required, and `--digestAlg` must match an
algorithm key in the attestation's in-toto subject.

### Keyless (public or private Sigstore)

Carry the certificate **and** the transparency log entries into the bundle. The
tlog is not optional here: Fulcio certificates are short-lived and will already be
expired by the time you verify, so the Rekor entry provides the trusted timestamp
that anchors the signature to the certificate's validity window. (Using
`--insecure-ignore-tlog` on a keyless attestation fails with *"expected a signed
timestamp to verify an expired certificate."*)

```bash
jq '{
    mediaType: "application/vnd.dev.sigstore.bundle.v0.3+json",
    verificationMaterial: {
        certificate: { rawBytes: .verification_material.certificate },
        tlogEntries:  .verification_material.transparency_entries
    },
    dsseEnvelope: {
        payload:     .envelope.statement,
        payloadType: "application/vnd.in-toto+json",
        signatures:  [ { sig: .envelope.signature } ]
    }
}' pep740.json > bundle.json

cosign initialize   # public instance; see below for a different one

cosign verify-blob-attestation \
    --bundle bundle.json \
    --certificate-identity "$IDENTITY" \
    --certificate-oidc-issuer "$OIDC_ISSUER" \
    --type "$PREDICATE_TYPE" \
    --digest "$DIGEST" --digestAlg "$DIGESTALG"
```

`$IDENTITY` is the signing workflow (e.g.
`https://github.com/OWNER/REPO/.github/workflows/publish.yml@refs/heads/main`) and
`$OIDC_ISSUER` is usually `https://token.actions.githubusercontent.com`.
`$PREDICATE_TYPE` for PyPI publish attestations is
`https://docs.pypi.org/attestations/publish/v1`.

**Different Sigstore instance.** If the certificate chains to a private Fulcio/Rekor
rather than the public one, run `cosign initialize` against your own instance's
trust root before verifying — for example:

```bash
cosign initialize --mirror "$YOUR_TUF_MIRROR" --root "$YOUR_TUF_ROOT"
```

The `verify-blob-attestation` command is otherwise unchanged. This is exactly the
case `pypi-attestations` cannot handle, because it always uses the public instance.

### Long-lived key

There is no certificate, so the bundle uses the `publicKey` variant and the
signature is checked directly against a known key. A long-lived key may or may not
also be logged in a transparency log, so carry `transparency_entries` into the
bundle when they are present:

```bash
jq '{
    mediaType: "application/vnd.dev.sigstore.bundle.v0.3+json",
    verificationMaterial: (
        { publicKey: { hint: "" } }
        + (if .verification_material.transparency_entries
           then { tlogEntries: .verification_material.transparency_entries }
           else {} end)
    ),
    dsseEnvelope: {
        payload:     .envelope.statement,
        payloadType: "application/vnd.in-toto+json",
        signatures:  [ { sig: .envelope.signature } ]
    }
}' pep740.json > bundle.json
```

If the attestation **is** logged in Rekor, verify the transparency log entry
(initialize cosign against the instance whose Rekor signed it, as above):

```bash
cosign verify-blob-attestation \
    --bundle bundle.json \
    --key signer.pub \
    --type "$PREDICATE_TYPE" \
    --digest "$DIGEST" --digestAlg "$DIGESTALG"
```

If it is **not** logged, there is nothing to anchor a timestamp — which is fine for
a long-lived key, since it has no expiry — so skip the tlog check explicitly:

```bash
cosign verify-blob-attestation \
    --bundle bundle.json \
    --key signer.pub --insecure-ignore-tlog \
    --type "$PREDICATE_TYPE" \
    --digest "$DIGEST" --digestAlg "$DIGESTALG"
```

The `hint` is unused because the key is supplied directly via `--key`.
