package forage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	provJSON := `{"version":1,"attestation_bundles":[{"bundle":"data"}]}`
	srv := newTestServer(t, provJSON)
	defer srv.Close()

	// We need the index HTML to have a provenance URL that points to our server.
	// Override the mux to serve HTML with the correct provenance URL.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /simple/requests/", func(w http.ResponseWriter, r *http.Request) {
		html := `<a href="/files/requests-2.32.2.tar.gz#sha256=ddeeff"
			data-provenance="` + srv.URL + `/provenance/requests-2.32.2.tar.gz">requests-2.32.2.tar.gz</a>`
		w.Write([]byte(html))
	})
	mux.HandleFunc("GET /provenance/requests-2.32.2.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(provJSON))
	})
	provSrv := httptest.NewServer(mux)
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
	if result.Files[0].Provenance == nil {
		t.Fatal("expected provenance data")
	}
	var prov map[string]any
	if err := json.Unmarshal(*result.Files[0].Provenance, &prov); err != nil {
		t.Fatalf("invalid provenance JSON: %v", err)
	}
	if prov["version"] != float64(1) {
		t.Errorf("provenance version = %v", prov["version"])
	}
}
