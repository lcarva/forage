package forage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestLookup_BasicAuthProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "__token__" || password != "secret" {
			t.Error("request did not contain expected credentials")
		}
		w.Write([]byte(`<a href="/files/pkg-1.0.tar.gz#sha256=abc123">pkg-1.0.tar.gz</a>`))
	}))
	defer srv.Close()

	result, err := Lookup(context.Background(), "pkg", "1.0", &Options{
		IndexURL: srv.URL,
		CredentialProvider: BasicAuthProvider{
			Origin:   srv.URL,
			Username: "__token__",
			Password: "secret",
		},
	})
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(result.Files))
	}
}

func TestLookup_Netrc(t *testing.T) {
	dir := t.TempDir()
	netrc := filepath.Join(dir, "netrc")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, password, ok := r.BasicAuth(); !ok || user != "user" || password != "pass" {
			t.Error("request did not contain netrc credentials")
		}
		w.Write([]byte(`<a href="/pkg-1.0.tar.gz#sha256=abc123">pkg-1.0.tar.gz</a>`))
	}))
	defer server.Close()

	serverURL := server.URL
	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(netrc, []byte("machine "+u.Host+" login user password pass\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Lookup(context.Background(), "pkg", "1.0", &Options{
		IndexURL:   serverURL,
		NetrcPath:  netrc,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(result.Files))
	}
}

func TestReadNetrc_DefaultAndQuotedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "netrc")
	if err := os.WriteFile(path, []byte("default login \"default-user\" password default-pass\nmachine example.com login user password \"pass word\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := readNetrc(path)
	if err != nil {
		t.Fatal(err)
	}
	if entries["default"].Username != "default-user" {
		t.Errorf("default username = %q", entries["default"].Username)
	}
	if entries["example.com"].Password != "pass word" {
		t.Errorf("machine password = %q", entries["example.com"].Password)
	}
}

func TestBasicAuthProvider_EmptyOriginFailsClosed(t *testing.T) {
	provider := BasicAuthProvider{Username: "user", Password: "pass"}
	target, err := url.Parse("https://packages.example.com/pkg")
	if err != nil {
		t.Fatal(err)
	}
	creds, ok, err := provider.Credentials(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if ok || creds != (Credentials{}) {
		t.Fatalf("empty origin acted as a wildcard: %+v, %v", creds, ok)
	}
}

func TestNetrcProvider_DefaultEntryIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "netrc")
	if err := os.WriteFile(path, []byte("default login user password pass\n"), 0600); err != nil {
		t.Fatal(err)
	}
	provider := &netrcProvider{path: path, explicit: true}
	target, err := url.Parse("https://pypi.org/simple/")
	if err != nil {
		t.Fatal(err)
	}
	creds, ok, err := provider.Credentials(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if ok || creds != (Credentials{}) {
		t.Fatalf("default netrc entry leaked credentials: %+v, %v", creds, ok)
	}
}

func TestNetrcProvider_ImplicitMalformedIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "netrc")
	if err := os.WriteFile(path, []byte("machine example.com login user password \"unterminated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	provider := &netrcProvider{path: path, explicit: false}
	target, err := url.Parse("https://example.com/pkg")
	if err != nil {
		t.Fatal(err)
	}
	creds, ok, err := provider.Credentials(context.Background(), target)
	if err != nil {
		t.Fatalf("implicit malformed netrc should not error: %v", err)
	}
	if ok || creds != (Credentials{}) {
		t.Fatalf("expected no credentials, got %+v, %v", creds, ok)
	}
}

func TestNetrcProvider_ExplicitMalformedErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "netrc")
	if err := os.WriteFile(path, []byte("machine example.com login user password \"unterminated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	provider := &netrcProvider{path: path, explicit: true}
	target, err := url.Parse("https://example.com/pkg")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := provider.Credentials(context.Background(), target); err == nil {
		t.Fatal("expected error for explicit malformed netrc, got nil")
	}
}

func TestCredentialsAreScopedToOrigin(t *testing.T) {
	provider := BasicAuthProvider{Origin: "https://packages.example.com", Username: "user", Password: "pass"}
	target, err := url.Parse("https://other.example.com/pkg")
	if err != nil {
		t.Fatal(err)
	}
	creds, ok, err := provider.Credentials(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if ok || creds != (Credentials{}) {
		t.Fatalf("credentials leaked to another origin: %+v, %v", creds, ok)
	}
}
