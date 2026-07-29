package forage

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatProvenance(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name: "full publisher",
			input: `{
				"version": 1,
				"attestation_bundles": [{
					"publisher": {
						"kind": "GitHub",
						"repository": "pypa/sampleproject",
						"workflow": "release.yml"
					},
					"attestations": [{"version": 1}, {"version": 1}]
				}]
			}`,
			contains: []string{
				"publisher: GitHub (pypa/sampleproject via release.yml)",
				"attestations: 2",
				"(use --json for full provenance data)",
			},
		},
		{
			name: "kind only",
			input: `{
				"version": 1,
				"attestation_bundles": [{
					"publisher": {"kind": "GitHub"},
					"attestations": [{"version": 1}]
				}]
			}`,
			contains: []string{
				"publisher: GitHub",
				"attestations: 1",
			},
		},
		{
			name: "no publisher fields",
			input: `{
				"version": 1,
				"attestation_bundles": [{
					"publisher": {},
					"attestations": [{"version": 1}]
				}]
			}`,
			contains: []string{
				"publisher: (unknown)",
			},
		},
		{
			name:  "empty bundles",
			input: `{"version": 1, "attestation_bundles": []}`,
			contains: []string{
				"(no attestation bundles)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := json.RawMessage(tt.input)
			result := FormatProvenance(&raw)
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

func TestFormatProvenance_Malformed(t *testing.T) {
	raw := json.RawMessage(`not json`)
	result := FormatProvenance(&raw)
	if result != "(unable to parse provenance data)" {
		t.Errorf("FormatProvenance(malformed) = %q", result)
	}
}

func TestFormatProvenance_Npm(t *testing.T) {
	input := `{
		"attestations": [
			{
				"predicateType": "https://slsa.dev/provenance/v1",
				"bundle": {}
			},
			{
				"predicateType": "https://github.com/npm/attestation/tree/main/specs/publish/v0.1",
				"bundle": {}
			}
		]
	}`
	raw := json.RawMessage(input)
	result := FormatProvenance(&raw)

	want := []string{
		"attestations: 2",
		"https://slsa.dev/provenance/v1",
		"https://github.com/npm/attestation/tree/main/specs/publish/v0.1",
		"(use --json for full provenance data)",
	}
	for _, w := range want {
		if !strings.Contains(result, w) {
			t.Errorf("FormatProvenance() = %q, missing %q", result, w)
		}
	}
}

func TestFormatProvenance_NpmSingleAttestation(t *testing.T) {
	input := `{
		"attestations": [
			{
				"predicateType": "https://slsa.dev/provenance/v1",
				"bundle": {}
			}
		]
	}`
	raw := json.RawMessage(input)
	result := FormatProvenance(&raw)

	if !strings.Contains(result, "attestations: 1") {
		t.Errorf("FormatProvenance() = %q, missing attestations count", result)
	}
}
