package forage

import (
	"strings"
	"testing"
)

const sampleHTML = `
<!DOCTYPE html>
<html><body>
<a href="https://files.example.com/requests-2.32.2-py3-none-any.whl#sha256=aabbcc">requests-2.32.2-py3-none-any.whl</a>
<a href="https://files.example.com/requests-2.32.2.tar.gz#sha256=ddeeff"
   data-provenance="https://example.com/provenance/requests-2.32.2.tar.gz">requests-2.32.2.tar.gz</a>
<a href="https://files.example.com/requests-2.32.3.tar.gz#sha256=112233">requests-2.32.3.tar.gz</a>
</body></html>
`

func TestParseSimpleIndex(t *testing.T) {
	files, err := parseSimpleIndex(strings.NewReader(sampleHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("got %d files, want 3", len(files))
	}

	// First file: wheel, no provenance
	if files[0].Filename != "requests-2.32.2-py3-none-any.whl" {
		t.Errorf("file[0].Filename = %q", files[0].Filename)
	}
	if len(files[0].Digests) != 1 || files[0].Digests[0].Algorithm != "sha256" || files[0].Digests[0].Value != "aabbcc" {
		t.Errorf("file[0].Digests = %v", files[0].Digests)
	}
	if files[0].ProvenanceURL != nil {
		t.Errorf("file[0].ProvenanceURL = %q, want nil", *files[0].ProvenanceURL)
	}

	// Second file: sdist with provenance
	if files[1].Filename != "requests-2.32.2.tar.gz" {
		t.Errorf("file[1].Filename = %q", files[1].Filename)
	}
	if len(files[1].Digests) != 1 || files[1].Digests[0].Algorithm != "sha256" || files[1].Digests[0].Value != "ddeeff" {
		t.Errorf("file[1].Digests = %v", files[1].Digests)
	}
	wantProv := "https://example.com/provenance/requests-2.32.2.tar.gz"
	if files[1].ProvenanceURL == nil || *files[1].ProvenanceURL != wantProv {
		t.Errorf("file[1].ProvenanceURL = %v, want %q", files[1].ProvenanceURL, wantProv)
	}

	// Third file
	if files[2].Filename != "requests-2.32.3.tar.gz" {
		t.Errorf("file[2].Filename = %q", files[2].Filename)
	}
	if len(files[2].Digests) != 1 || files[2].Digests[0].Algorithm != "sha256" || files[2].Digests[0].Value != "112233" {
		t.Errorf("file[2].Digests = %v", files[2].Digests)
	}
}

func TestParseSimpleIndex_NoFragment(t *testing.T) {
	html := `<a href="https://example.com/pkg-1.0.tar.gz">pkg-1.0.tar.gz</a>`
	files, err := parseSimpleIndex(strings.NewReader(html))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	if len(files[0].Digests) != 0 {
		t.Errorf("Digests = %v, want empty", files[0].Digests)
	}
}

func TestParseSimpleIndex_Empty(t *testing.T) {
	files, err := parseSimpleIndex(strings.NewReader("<html><body></body></html>"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("got %d files, want 0", len(files))
	}
}
