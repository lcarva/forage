package forage

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const indexHTML = `
<!DOCTYPE html>
<html><body>
<a href="/files/requests-2.32.2-py3-none-any.whl#sha256=aabbcc">requests-2.32.2-py3-none-any.whl</a>
<a href="/files/requests-2.32.2.tar.gz#sha256=ddeeff"
   data-provenance="PROVENANCE_URL">requests-2.32.2.tar.gz</a>
<a href="/files/requests-2.32.3.tar.gz#sha256=112233">requests-2.32.3.tar.gz</a>
</body></html>
`

func newTestServer(t *testing.T, provenanceBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /simple/requests/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(indexHTML))
	})
	mux.HandleFunc("GET /simple/nonexistent/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("GET /provenance/requests-2.32.2.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(provenanceBody))
	})
	return httptest.NewServer(mux)
}

func TestLookup(t *testing.T) {
	srv := newTestServer(t, "")
	defer srv.Close()

	// Rewrite provenance URL in the HTML to point to our test server
	origHTML := indexHTML
	_ = origHTML

	opts := &Options{
		IndexURL:   srv.URL + "/simple/",
		HTTPClient: srv.Client(),
	}
	result, err := Lookup(context.Background(), "requests", "2.32.2", opts)
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if result.Package != "requests" {
		t.Errorf("Package = %q", result.Package)
	}
	if result.Version != "2.32.2" {
		t.Errorf("Version = %q", result.Version)
	}
	if len(result.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(result.Files))
	}
	if result.Files[0].Filename != "requests-2.32.2-py3-none-any.whl" {
		t.Errorf("file[0].Filename = %q", result.Files[0].Filename)
	}
	if result.Files[1].Filename != "requests-2.32.2.tar.gz" {
		t.Errorf("file[1].Filename = %q", result.Files[1].Filename)
	}
}

func TestLookup_NotFound(t *testing.T) {
	srv := newTestServer(t, "")
	defer srv.Close()

	opts := &Options{
		IndexURL:   srv.URL + "/simple/",
		HTTPClient: srv.Client(),
	}
	_, err := Lookup(context.Background(), "nonexistent", "1.0", opts)
	if err == nil {
		t.Fatal("expected error for nonexistent package")
	}
}

func TestLookup_NoVersionMatch(t *testing.T) {
	srv := newTestServer(t, "")
	defer srv.Close()

	opts := &Options{
		IndexURL:   srv.URL + "/simple/",
		HTTPClient: srv.Client(),
	}
	_, err := Lookup(context.Background(), "requests", "9.9.9", opts)
	if err == nil {
		t.Fatal("expected error for unmatched version")
	}
}

func TestLookup_FetchProvenance(t *testing.T) {
	stmt := `{"predicateType":"https://docs.pypi.org/attestations/publish/v1"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(stmt))
	provJSON := `{"version":1,"attestation_bundles":[{
		"publisher":{"kind":"GitHub","repository":"psf/requests","workflow":"release.yml"},
		"attestations":[{"version":1,"envelope":{"statement":"` + encoded + `","signature":"sig"},"verification_material":{}}]
	}]}`

	mux := http.NewServeMux()
	provSrv := httptest.NewUnstartedServer(mux)
	mux.HandleFunc("GET /simple/requests/", func(w http.ResponseWriter, r *http.Request) {
		html := `<a href="/files/requests-2.32.2.tar.gz#sha256=ddeeff"
			data-provenance="` + provSrv.URL + `/provenance/requests-2.32.2.tar.gz">requests-2.32.2.tar.gz</a>`
		w.Write([]byte(html))
	})
	mux.HandleFunc("GET /provenance/requests-2.32.2.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(provJSON))
	})
	provSrv.Start()
	defer provSrv.Close()

	opts := &Options{
		IndexURL:        provSrv.URL + "/simple/",
		FetchProvenance: true,
		HTTPClient:      provSrv.Client(),
	}
	result, err := Lookup(context.Background(), "requests", "2.32.2", opts)
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(result.Files))
	}
	prov := result.Files[0].Provenance
	if prov == nil {
		t.Fatal("expected provenance data")
	}
	if prov.Publisher == nil || prov.Publisher.Kind != "GitHub" {
		t.Errorf("expected GitHub publisher, got %+v", prov.Publisher)
	}
	if len(prov.Attestations) != 1 {
		t.Fatalf("got %d attestations, want 1", len(prov.Attestations))
	}
	if prov.Attestations[0].PredicateType != "https://docs.pypi.org/attestations/publish/v1" {
		t.Errorf("predicateType = %q", prov.Attestations[0].PredicateType)
	}
}

func TestLookup_FetchProvenance_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /simple/requests/", func(w http.ResponseWriter, r *http.Request) {
		html := `<a href="/files/requests-2.32.2.tar.gz#sha256=ddeeff"
			data-provenance="http://unreachable.invalid/provenance">requests-2.32.2.tar.gz</a>`
		w.Write([]byte(html))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	opts := &Options{
		IndexURL:        srv.URL + "/simple/",
		FetchProvenance: true,
		HTTPClient:      srv.Client(),
	}
	result, err := Lookup(context.Background(), "requests", "2.32.2", opts)
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(result.Files))
	}
	f := result.Files[0]
	if f.Provenance != nil {
		t.Error("expected Provenance to be nil on fetch error")
	}
	if f.ProvenanceError == nil {
		t.Fatal("expected ProvenanceError to be set")
	}
	if *f.ProvenanceError == "" {
		t.Error("ProvenanceError should not be empty")
	}
}

func TestFetchIndex_AcceptHeader(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		if gotAccept == "" || gotAccept == "*/*" {
			w.Header().Set("Content-Type", "application/vnd.pypi.simple.v1+json")
			w.Write([]byte(`{"files": []}`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<a href="https://example.com/pkg-1.0.tar.gz#sha256=abc123">pkg-1.0.tar.gz</a>`))
	}))
	defer srv.Close()

	_, err := Lookup(context.Background(), "pkg", "1.0", &Options{
		IndexURL:   srv.URL,
		HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatalf("Lookup failed against PEP 691 server: %v", err)
	}
	if gotAccept == "" || gotAccept == "*/*" {
		t.Error("fetchIndex did not set an Accept header; PEP 691 servers may return JSON instead of HTML")
	}
}

func TestParseSRI(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantAlg   string
		wantValue string
		wantErr   bool
	}{
		{
			name:      "valid sha512",
			input:     "sha512-AQID",
			wantAlg:   "sha512",
			wantValue: "010203",
		},
		{
			name:      "valid sha256",
			input:     "sha256-AQID",
			wantAlg:   "sha256",
			wantValue: "010203",
		},
		{
			name:    "missing hyphen",
			input:   "sha512AQID",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid base64",
			input:   "sha512-!!!notbase64",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := ParseSRI(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.Algorithm != tt.wantAlg {
				t.Errorf("Algorithm = %q, want %q", d.Algorithm, tt.wantAlg)
			}
			if d.Value != tt.wantValue {
				t.Errorf("Value = %q, want %q", d.Value, tt.wantValue)
			}
		})
	}
}

func TestLookup_FetchProvenance_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	mux.HandleFunc("GET /simple/requests/", func(w http.ResponseWriter, r *http.Request) {
		html := `<a href="/files/requests-2.32.2.tar.gz#sha256=ddeeff"
			data-provenance="` + srv.URL + `/provenance/requests-2.32.2.tar.gz">requests-2.32.2.tar.gz</a>`
		w.Write([]byte(html))
	})
	mux.HandleFunc("GET /provenance/requests-2.32.2.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	})
	defer srv.Close()

	opts := &Options{
		IndexURL:        srv.URL + "/simple/",
		FetchProvenance: true,
		HTTPClient:      srv.Client(),
	}
	result, err := Lookup(context.Background(), "requests", "2.32.2", opts)
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(result.Files))
	}
	f := result.Files[0]
	if f.Provenance != nil {
		t.Error("expected Provenance to be nil on server error")
	}
	if f.ProvenanceError == nil {
		t.Fatal("expected ProvenanceError to be set")
	}
	if !strings.Contains(*f.ProvenanceError, "500") {
		t.Errorf("ProvenanceError = %q, want it to mention status 500", *f.ProvenanceError)
	}
}
