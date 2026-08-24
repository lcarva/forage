package forage

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

const mediaTypePyPIAttestation = "application/vnd.pypi.attestation+json"

type provenanceEnvelope struct {
	Version            int                 `json:"version"`
	AttestationBundles []attestationBundle `json:"attestation_bundles"`
}

type attestationBundle struct {
	Attestations []json.RawMessage `json:"attestations"`
}

type npmAttestationResponse struct {
	Attestations []npmAttestation `json:"attestations"`
}

type npmAttestation struct {
	PredicateType string          `json:"predicateType"`
	Bundle        json.RawMessage `json:"bundle"`
}

func normalizePyPIProvenance(raw []byte) (*Provenance, error) {
	var env provenanceEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("parsing PyPI provenance: %w", err)
	}

	var attestations []Attestation
	for _, bundle := range env.AttestationBundles {
		for _, raw := range bundle.Attestations {
			attestations = append(attestations, Attestation{
				MediaType:     mediaTypePyPIAttestation,
				PredicateType: extractPredicateType(raw),
				Bundle:        raw,
			})
		}
	}

	return &Provenance{
		Attestations: attestations,
	}, nil
}

func normalizeNpmProvenance(raw []byte) (*Provenance, error) {
	var resp npmAttestationResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parsing npm provenance: %w", err)
	}

	var attestations []Attestation
	for _, a := range resp.Attestations {
		attestations = append(attestations, Attestation{
			MediaType:     extractBundleMediaType(a.Bundle),
			PredicateType: a.PredicateType,
			Bundle:        a.Bundle,
		})
	}

	return &Provenance{Attestations: attestations}, nil
}

func extractBundleMediaType(bundle json.RawMessage) string {
	var m struct {
		MediaType string `json:"mediaType"`
	}
	if err := json.Unmarshal(bundle, &m); err == nil && m.MediaType != "" {
		return m.MediaType
	}
	return "application/vnd.dev.sigstore.bundle+json"
}

func extractPredicateType(attestation json.RawMessage) string {
	var att struct {
		Envelope struct {
			Statement string `json:"statement"`
		} `json:"envelope"`
	}
	if err := json.Unmarshal(attestation, &att); err != nil || att.Envelope.Statement == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(att.Envelope.Statement)
	if err != nil {
		return ""
	}
	var stmt struct {
		PredicateType string `json:"predicateType"`
	}
	if err := json.Unmarshal(decoded, &stmt); err != nil {
		return ""
	}
	return stmt.PredicateType
}

// FormatProvenance returns a human-readable summary of provenance data.
func FormatProvenance(p *Provenance) string {
	if p == nil {
		return "(no provenance data)"
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("attestations: %d", len(p.Attestations)))
	for _, a := range p.Attestations {
		if a.PredicateType != "" {
			lines = append(lines, fmt.Sprintf("  - %s", a.PredicateType))
		}
	}

	if len(p.Attestations) == 0 {
		lines = append(lines, "(no attestation bundles)")
	}

	lines = append(lines, "(use --json for full provenance data)")
	return strings.Join(lines, "\n")
}
