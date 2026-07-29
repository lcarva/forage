package forage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sigstoreVersionJSON = `{
	"name": "sigstore",
	"version": "2.3.1",
	"dist": {
		"integrity": "sha512-abcdef1234567890==",
		"shasum": "deadbeef12345678",
		"tarball": "TARBALL_URL/sigstore/-/sigstore-2.3.1.tgz"
	}
}`

func newNpmTestServer(t *testing.T, attestationBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)

	versionJSON := strings.ReplaceAll(sigstoreVersionJSON, "TARBALL_URL", srv.URL)

	mux.HandleFunc("GET /sigstore/2.3.1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(versionJSON))
	})
	mux.HandleFunc("GET /@scope/mypkg/1.0.0", func(w http.ResponseWriter, r *http.Request) {
		scopedJSON := `{
			"name": "@scope/mypkg",
			"version": "1.0.0",
			"dist": {
				"integrity": "sha512-scopedhash==",
				"shasum": "scopedshasum",
				"tarball": "` + srv.URL + `/@scope/mypkg/-/mypkg-1.0.0.tgz"
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(scopedJSON))
	})
	mux.HandleFunc("GET /nonexistent/1.0.0", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("GET /-/npm/v1/attestations/sigstore@2.3.1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(attestationBody))
	})
	return srv
}

func TestNpmLookup(t *testing.T) {
	srv := newNpmTestServer(t, "")
	defer srv.Close()

	opts := &Options{
		IndexURL:   srv.URL,
		HTTPClient: srv.Client(),
	}
	result, err := NpmLookup(context.Background(), "sigstore", "2.3.1", opts)
	if err != nil {
		t.Fatalf("NpmLookup failed: %v", err)
	}
	if result.Package != "sigstore" {
		t.Errorf("Package = %q", result.Package)
	}
	if result.Version != "2.3.1" {
		t.Errorf("Version = %q", result.Version)
	}
	if len(result.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(result.Files))
	}
	f := result.Files[0]
	if f.Filename != "sigstore-2.3.1.tgz" {
		t.Errorf("Filename = %q", f.Filename)
	}
	if f.Integrity != "sha512-abcdef1234567890==" {
		t.Errorf("Integrity = %q", f.Integrity)
	}
	if f.Shasum != "deadbeef12345678" {
		t.Errorf("Shasum = %q", f.Shasum)
	}
}

func TestNpmLookup_ScopedPackage(t *testing.T) {
	srv := newNpmTestServer(t, "")
	defer srv.Close()

	opts := &Options{
		IndexURL:   srv.URL,
		HTTPClient: srv.Client(),
	}
	result, err := NpmLookup(context.Background(), "@scope/mypkg", "1.0.0", opts)
	if err != nil {
		t.Fatalf("NpmLookup failed: %v", err)
	}
	if result.Package != "@scope/mypkg" {
		t.Errorf("Package = %q", result.Package)
	}
	if len(result.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(result.Files))
	}
	if result.Files[0].Filename != "mypkg-1.0.0.tgz" {
		t.Errorf("Filename = %q", result.Files[0].Filename)
	}
}

func TestNpmLookup_NotFound(t *testing.T) {
	srv := newNpmTestServer(t, "")
	defer srv.Close()

	opts := &Options{
		IndexURL:   srv.URL,
		HTTPClient: srv.Client(),
	}
	_, err := NpmLookup(context.Background(), "nonexistent", "1.0.0", opts)
	if err == nil {
		t.Fatal("expected error for nonexistent package")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to mention 'not found'", err.Error())
	}
}

func TestNpmLookup_DefaultIndex(t *testing.T) {
	opts := &Options{}
	if opts.IndexURL != "" {
		t.Fatal("expected empty IndexURL to use default")
	}
}

func TestNpmLookup_FetchProvenance(t *testing.T) {
	attJSON := `{
		"attestations": [
			{
				"predicateType": "https://slsa.dev/provenance/v1",
				"bundle": {"mediaType": "application/vnd.dev.sigstore.bundle.v0.3+json"}
			},
			{
				"predicateType": "https://github.com/npm/attestation/tree/main/specs/publish/v0.1",
				"bundle": {"mediaType": "application/vnd.dev.sigstore.bundle.v0.3+json"}
			}
		]
	}`
	srv := newNpmTestServer(t, attJSON)
	defer srv.Close()

	opts := &Options{
		IndexURL:        srv.URL,
		FetchProvenance: true,
		HTTPClient:      srv.Client(),
	}
	result, err := NpmLookup(context.Background(), "sigstore", "2.3.1", opts)
	if err != nil {
		t.Fatalf("NpmLookup failed: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(result.Files))
	}
	f := result.Files[0]
	if f.Provenance == nil {
		t.Fatal("expected provenance data")
	}
	var att map[string]any
	if err := json.Unmarshal(*f.Provenance, &att); err != nil {
		t.Fatalf("invalid provenance JSON: %v", err)
	}
	attestations, ok := att["attestations"].([]any)
	if !ok || len(attestations) != 2 {
		t.Errorf("expected 2 attestations, got %v", att["attestations"])
	}
}

func TestNpmLookup_FetchProvenance_Error(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	versionJSON := strings.ReplaceAll(sigstoreVersionJSON, "TARBALL_URL", srv.URL)
	mux.HandleFunc("GET /sigstore/2.3.1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(versionJSON))
	})
	mux.HandleFunc("GET /-/npm/v1/attestations/sigstore@2.3.1", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	})

	opts := &Options{
		IndexURL:        srv.URL,
		FetchProvenance: true,
		HTTPClient:      srv.Client(),
	}
	result, err := NpmLookup(context.Background(), "sigstore", "2.3.1", opts)
	if err != nil {
		t.Fatalf("NpmLookup failed: %v", err)
	}
	f := result.Files[0]
	if f.Provenance != nil {
		t.Error("expected Provenance to be nil on fetch error")
	}
	if f.ProvenanceError == nil {
		t.Fatal("expected ProvenanceError to be set")
	}
	if !strings.Contains(*f.ProvenanceError, "500") {
		t.Errorf("ProvenanceError = %q, want it to mention status 500", *f.ProvenanceError)
	}
}

func TestNpmAttestationURL(t *testing.T) {
	tests := []struct {
		indexURL string
		pkg      string
		version  string
		want     string
	}{
		{
			"https://registry.npmjs.org",
			"sigstore", "2.3.1",
			"https://registry.npmjs.org/-/npm/v1/attestations/sigstore@2.3.1",
		},
		{
			"https://registry.npmjs.org/",
			"@scope/pkg", "1.0.0",
			"https://registry.npmjs.org/-/npm/v1/attestations/@scope/pkg@1.0.0",
		},
	}
	for _, tt := range tests {
		got := npmAttestationURL(tt.indexURL, tt.pkg, tt.version)
		if got != tt.want {
			t.Errorf("npmAttestationURL(%q, %q, %q) = %q, want %q",
				tt.indexURL, tt.pkg, tt.version, got, tt.want)
		}
	}
}

func TestNpmLookup_AcceptHeader(t *testing.T) {
	var gotAccept string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /testpkg/1.0.0", func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"name": "testpkg",
			"version": "1.0.0",
			"dist": {
				"tarball": "https://example.com/testpkg-1.0.0.tgz",
				"integrity": "sha512-abc==",
				"shasum": "def"
			}
		}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, err := NpmLookup(context.Background(), "testpkg", "1.0.0", &Options{
		IndexURL:   srv.URL,
		HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatalf("NpmLookup failed: %v", err)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept header = %q, want %q", gotAccept, "application/json")
	}
}
