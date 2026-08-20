package forage

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizePyPIProvenance(t *testing.T) {
	stmt := `{"_type":"https://in-toto.io/Statement/v1","predicateType":"https://docs.pypi.org/attestations/publish/v1","subject":[]}`
	encoded := base64.StdEncoding.EncodeToString([]byte(stmt))

	input := `{
		"version": 1,
		"attestation_bundles": [{
			"publisher": {
				"kind": "GitHub",
				"repository": "pypa/sampleproject",
				"workflow": "release.yml"
			},
			"attestations": [
				{"version": 1, "envelope": {"statement": "` + encoded + `", "signature": "sig"}, "verification_material": {}}
			]
		}]
	}`

	prov, err := normalizePyPIProvenance([]byte(input))
	if err != nil {
		t.Fatalf("normalizePyPIProvenance() error: %v", err)
	}
	if prov.Publisher == nil {
		t.Fatal("expected publisher")
	}
	if prov.Publisher.Kind != "GitHub" {
		t.Errorf("publisher.Kind = %q, want GitHub", prov.Publisher.Kind)
	}
	if prov.Publisher.Repository != "pypa/sampleproject" {
		t.Errorf("publisher.Repository = %q", prov.Publisher.Repository)
	}
	if len(prov.Attestations) != 1 {
		t.Fatalf("got %d attestations, want 1", len(prov.Attestations))
	}
	a := prov.Attestations[0]
	if a.MediaType != mediaTypePyPIAttestation {
		t.Errorf("mediaType = %q, want %q", a.MediaType, mediaTypePyPIAttestation)
	}
	if a.PredicateType != "https://docs.pypi.org/attestations/publish/v1" {
		t.Errorf("predicateType = %q", a.PredicateType)
	}
	if a.Bundle == nil {
		t.Error("expected bundle")
	}
}

func TestNormalizePyPIProvenance_MultipleBundles(t *testing.T) {
	input := `{
		"version": 1,
		"attestation_bundles": [
			{"publisher": {"kind": "GitHub"}, "attestations": [{"version": 1}]},
			{"publisher": {"kind": "GitLab"}, "attestations": [{"version": 1}, {"version": 1}]}
		]
	}`

	prov, err := normalizePyPIProvenance([]byte(input))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if prov.Publisher.Kind != "GitHub" {
		t.Errorf("expected first publisher, got %q", prov.Publisher.Kind)
	}
	if len(prov.Attestations) != 3 {
		t.Errorf("got %d attestations, want 3", len(prov.Attestations))
	}
}

func TestNormalizePyPIProvenance_NoStatement(t *testing.T) {
	input := `{
		"version": 1,
		"attestation_bundles": [{
			"publisher": {"kind": "GitHub"},
			"attestations": [{"version": 1, "envelope": {"signature": "sig"}}]
		}]
	}`

	prov, err := normalizePyPIProvenance([]byte(input))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if prov.Attestations[0].PredicateType != "" {
		t.Errorf("expected empty predicateType, got %q", prov.Attestations[0].PredicateType)
	}
}

func TestNormalizePyPIProvenance_Malformed(t *testing.T) {
	_, err := normalizePyPIProvenance([]byte(`not json`))
	if err == nil {
		t.Error("expected error for malformed input")
	}
}

func TestNormalizeNpmProvenance(t *testing.T) {
	input := `{
		"attestations": [
			{
				"predicateType": "https://slsa.dev/provenance/v1",
				"bundle": {"mediaType": "application/vnd.dev.sigstore.bundle.v0.3+json", "dsseEnvelope": {}}
			},
			{
				"predicateType": "https://github.com/npm/attestation/tree/main/specs/publish/v0.1",
				"bundle": {"mediaType": "application/vnd.dev.sigstore.bundle.v0.3+json", "dsseEnvelope": {}}
			}
		]
	}`

	prov, err := normalizeNpmProvenance([]byte(input))
	if err != nil {
		t.Fatalf("normalizeNpmProvenance() error: %v", err)
	}
	if prov.Publisher != nil {
		t.Error("expected nil publisher for npm")
	}
	if len(prov.Attestations) != 2 {
		t.Fatalf("got %d attestations, want 2", len(prov.Attestations))
	}
	a := prov.Attestations[0]
	if a.MediaType != "application/vnd.dev.sigstore.bundle.v0.3+json" {
		t.Errorf("mediaType = %q", a.MediaType)
	}
	if a.PredicateType != "https://slsa.dev/provenance/v1" {
		t.Errorf("predicateType = %q", a.PredicateType)
	}
}

func TestNormalizeNpmProvenance_NoMediaType(t *testing.T) {
	input := `{"attestations": [{"predicateType": "test", "bundle": {"dsseEnvelope": {}}}]}`

	prov, err := normalizeNpmProvenance([]byte(input))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if prov.Attestations[0].MediaType != "application/vnd.dev.sigstore.bundle+json" {
		t.Errorf("expected fallback mediaType, got %q", prov.Attestations[0].MediaType)
	}
}

func TestNormalizeNpmProvenance_Malformed(t *testing.T) {
	_, err := normalizeNpmProvenance([]byte(`not json`))
	if err == nil {
		t.Error("expected error for malformed input")
	}
}

func TestExtractPredicateType(t *testing.T) {
	stmt := `{"predicateType":"https://slsa.dev/provenance/v1"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(stmt))
	att := json.RawMessage(`{"envelope":{"statement":"` + encoded + `"}}`)

	pt := extractPredicateType(att)
	if pt != "https://slsa.dev/provenance/v1" {
		t.Errorf("extractPredicateType() = %q", pt)
	}
}

func TestExtractPredicateType_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no envelope", `{"version": 1}`},
		{"empty statement", `{"envelope":{"statement":""}}`},
		{"invalid base64", `{"envelope":{"statement":"!!!"}}`},
		{"invalid json in statement", `{"envelope":{"statement":"` + base64.StdEncoding.EncodeToString([]byte(`not json`)) + `"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := extractPredicateType(json.RawMessage(tt.input))
			if pt != "" {
				t.Errorf("expected empty, got %q", pt)
			}
		})
	}
}

func TestFormatProvenance(t *testing.T) {
	tests := []struct {
		name     string
		input    *Provenance
		contains []string
	}{
		{
			name: "full publisher with attestations",
			input: &Provenance{
				Publisher: &Publisher{
					Kind:       "GitHub",
					Repository: "pypa/sampleproject",
					Workflow:   "release.yml",
				},
				Attestations: []Attestation{
					{MediaType: mediaTypePyPIAttestation, PredicateType: "https://docs.pypi.org/attestations/publish/v1"},
					{MediaType: mediaTypePyPIAttestation, PredicateType: "https://slsa.dev/provenance/v1"},
				},
			},
			contains: []string{
				"publisher: GitHub (pypa/sampleproject via release.yml)",
				"attestations: 2",
				"https://docs.pypi.org/attestations/publish/v1",
				"https://slsa.dev/provenance/v1",
				"(use --json for full provenance data)",
			},
		},
		{
			name: "kind only publisher",
			input: &Provenance{
				Publisher:    &Publisher{Kind: "GitHub"},
				Attestations: []Attestation{{MediaType: mediaTypePyPIAttestation}},
			},
			contains: []string{
				"publisher: GitHub",
				"attestations: 1",
			},
		},
		{
			name: "no publisher (npm style)",
			input: &Provenance{
				Attestations: []Attestation{
					{MediaType: "application/vnd.dev.sigstore.bundle.v0.3+json", PredicateType: "https://slsa.dev/provenance/v1"},
				},
			},
			contains: []string{
				"attestations: 1",
				"https://slsa.dev/provenance/v1",
			},
		},
		{
			name:  "empty attestations",
			input: &Provenance{},
			contains: []string{
				"attestations: 0",
				"(no attestation bundles)",
			},
		},
		{
			name: "unknown publisher",
			input: &Provenance{
				Publisher:    &Publisher{},
				Attestations: []Attestation{{MediaType: mediaTypePyPIAttestation}},
			},
			contains: []string{
				"publisher: (unknown)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatProvenance(tt.input)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("FormatProvenance() = %q, missing %q", result, want)
				}
			}
		})
	}
}

func TestFormatProvenance_Nil(t *testing.T) {
	result := FormatProvenance(nil)
	if result != "(no provenance data)" {
		t.Errorf("FormatProvenance(nil) = %q", result)
	}
}
