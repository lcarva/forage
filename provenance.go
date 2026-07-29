package forage

import (
	"encoding/json"
	"fmt"
	"strings"
)

type provenanceEnvelope struct {
	Version            int                 `json:"version"`
	AttestationBundles []attestationBundle `json:"attestation_bundles"`
}

type attestationBundle struct {
	Publisher    publisher         `json:"publisher"`
	Attestations []json.RawMessage `json:"attestations"`
}

type publisher struct {
	Kind       string `json:"kind"`
	Repository string `json:"repository"`
	Workflow   string `json:"workflow"`
}

type npmAttestationResponse struct {
	Attestations []npmAttestation `json:"attestations"`
}

type npmAttestation struct {
	PredicateType string          `json:"predicateType"`
	Bundle        json.RawMessage `json:"bundle"`
}

// FormatProvenance returns a human-readable summary of provenance data.
func FormatProvenance(raw *json.RawMessage) string {
	if raw == nil {
		return "(no provenance data)"
	}

	var npmAtt npmAttestationResponse
	if err := json.Unmarshal(*raw, &npmAtt); err == nil && len(npmAtt.Attestations) > 0 {
		return formatNpmProvenance(npmAtt)
	}

	var env provenanceEnvelope
	if err := json.Unmarshal(*raw, &env); err != nil {
		return "(unable to parse provenance data)"
	}

	var lines []string
	for _, bundle := range env.AttestationBundles {
		pub := formatPublisher(bundle.Publisher)
		lines = append(lines, fmt.Sprintf("publisher: %s", pub))
		lines = append(lines, fmt.Sprintf("attestations: %d", len(bundle.Attestations)))
	}

	if len(lines) == 0 {
		lines = append(lines, "(no attestation bundles)")
	}

	lines = append(lines, "(use --json for full provenance data)")
	return strings.Join(lines, "\n")
}

func formatNpmProvenance(att npmAttestationResponse) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("attestations: %d", len(att.Attestations)))
	for _, a := range att.Attestations {
		lines = append(lines, fmt.Sprintf("  - %s", a.PredicateType))
	}
	lines = append(lines, "(use --json for full provenance data)")
	return strings.Join(lines, "\n")
}

func formatPublisher(p publisher) string {
	var parts []string
	if p.Repository != "" {
		parts = append(parts, p.Repository)
	}
	if p.Workflow != "" {
		parts = append(parts, "via "+p.Workflow)
	}

	if p.Kind != "" && len(parts) > 0 {
		return fmt.Sprintf("%s (%s)", p.Kind, strings.Join(parts, " "))
	}
	if p.Kind != "" {
		return p.Kind
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	return "(unknown)"
}
